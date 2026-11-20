package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stoodify_backend/migrations"
)

// mailerUji menangkap token verifikasi supaya bisa dipakai di request berikutnya.
type mailerUji struct {
	mu       sync.Mutex
	terakhir string
}

func (m *mailerUji) SendPasswordReset(toEmail, resetToken string) error { return nil }

func (m *mailerUji) SendEmailVerification(toEmail, verifyToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terakhir = verifyToken
	return nil
}

func (m *mailerUji) token() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.terakhir
}

// tungguToken menunggu email verifikasi terkirim (pengiriman berjalan async).
func tungguToken(t *testing.T, m *mailerUji) string {
	t.Helper()
	require.Eventually(t, func() bool { return m.token() != "" }, 2*time.Second, 10*time.Millisecond,
		"email verifikasi tidak pernah dikirim")
	return m.token()
}

func setupVerifikasi(t *testing.T) (*gin.Engine, *mailerUji) {
	t.Helper()
	db := setupAITestDB(t)

	m := &mailerUji{}
	auth := NewAuthController(db)
	auth.Mailer = m

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/register", auth.Register)
	r.POST("/auth/login", auth.Login)
	r.POST("/auth/verify-email", auth.VerifyEmail)
	r.POST("/auth/verify-email/resend", auth.ResendVerification)
	return r, m
}

func postJSONAuth(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func daftar(t *testing.T, r *gin.Engine, email string) {
	t.Helper()
	body := `{"name":"Siswa Baru","email":"` + email + `","school":"SMK Telkom","grade":10,"major":"SIJA","password":"rahasia123","confirm_password":"rahasia123"}`
	w := postJSONAuth(r, "/auth/register", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestRegisterMembuatAkunBelumTerverifikasi(t *testing.T) {
	r, m := setupVerifikasi(t)
	daftar(t, r, "baru@example.com")

	var user migrations.User
	require.NoError(t, configDB().Where("email = ?", "baru@example.com").First(&user).Error)
	assert.False(t, user.EmailVerified, "akun baru harus belum terverifikasi")

	var jumlah int64
	require.NoError(t, configDB().Model(&migrations.EmailVerificationToken{}).Where("user_id = ?", user.ID).Count(&jumlah).Error)
	assert.Equal(t, int64(1), jumlah, "harus ada satu token verifikasi")

	// Token dikirim lewat mailer (bukan disimpan mentah di DB).
	require.NotEmpty(t, tungguToken(t, m))
}

func TestLoginDitolakSebelumVerifikasi(t *testing.T) {
	r, _ := setupVerifikasi(t)
	daftar(t, r, "belum@example.com")

	w := postJSONAuth(r, "/auth/login", `{"email":"belum@example.com","password":"rahasia123"}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "false", resp.Errors["email_verified"])
	assert.Contains(t, w.Body.String(), "Email belum diverifikasi")
}

func TestVerifikasiEmailMengaktifkanLogin(t *testing.T) {
	r, m := setupVerifikasi(t)
	daftar(t, r, "aktif@example.com")

	w := postJSONAuth(r, "/auth/verify-email", `{"token":"`+tungguToken(t, m)+`"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = postJSONAuth(r, "/auth/login", `{"email":"aktif@example.com","password":"rahasia123"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "access_token")

	var user migrations.User
	require.NoError(t, configDB().Where("email = ?", "aktif@example.com").First(&user).Error)
	assert.True(t, user.EmailVerified)
}

func TestTokenVerifikasiSekaliPakai(t *testing.T) {
	r, m := setupVerifikasi(t)
	daftar(t, r, "sekali@example.com")
	token := tungguToken(t, m)

	require.Equal(t, http.StatusOK, postJSONAuth(r, "/auth/verify-email", `{"token":"`+token+`"}`).Code)

	w := postJSONAuth(r, "/auth/verify-email", `{"token":"`+token+`"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, "token yang sama tidak boleh dipakai dua kali")
}

func TestTokenVerifikasiKedaluwarsa(t *testing.T) {
	r, m := setupVerifikasi(t)
	daftar(t, r, "kadaluwarsa@example.com")

	require.NoError(t, configDB().Model(&migrations.EmailVerificationToken{}).
		Where("used_at IS NULL").Update("expires_at", time.Now().Add(-time.Minute)).Error)

	w := postJSONAuth(r, "/auth/verify-email", `{"token":"`+tungguToken(t, m)+`"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "kedaluwarsa")
}

func TestResendVerifikasiMenggantiTokenLama(t *testing.T) {
	r, m := setupVerifikasi(t)
	daftar(t, r, "resend@example.com")
	lama := tungguToken(t, m)

	w := postJSONAuth(r, "/auth/verify-email/resend", `{"email":"resend@example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var baru string
	require.Eventually(t, func() bool {
		baru = m.token()
		return baru != "" && baru != lama
	}, 2*time.Second, 10*time.Millisecond, "harus ada token baru")
	require.NotEqual(t, lama, baru, "harus ada token baru")

	// Token lama dibatalkan.
	assert.Equal(t, http.StatusBadRequest, postJSONAuth(r, "/auth/verify-email", `{"token":"`+lama+`"}`).Code)
	assert.Equal(t, http.StatusOK, postJSONAuth(r, "/auth/verify-email", `{"token":"`+baru+`"}`).Code)
}

func TestResendVerifikasiSelaluBalasanGenerik(t *testing.T) {
	r, _ := setupVerifikasi(t)

	w := postJSONAuth(r, "/auth/verify-email/resend", `{"email":"tidak-ada@example.com"}`)
	require.Equal(t, http.StatusOK, w.Code, "email tidak terdaftar tetap balasan sama")
	assert.Contains(t, w.Body.String(), "Jika email terdaftar")
}

func TestGoogleLoginEmailSudahTerverifikasi(t *testing.T) {
	r, key, _ := setupGoogle(t)

	w := panggilGoogle(r, tokenGoogle(t, key, klaimGoogle{
		Sub: "sub-verif", Email: "google-verif@example.com", Name: "Google User",
	}))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var user migrations.User
	require.NoError(t, configDB().Where("email = ?", "google-verif@example.com").First(&user).Error)
	assert.True(t, user.EmailVerified, "email dari Google sudah terverifikasi")
}

// Login tanpa id_token harus tetap menuntut email+password (422, bukan 400).
func TestLoginTanpaIDTokenDanTanpaPassword(t *testing.T) {
	r, _ := setupVerifikasi(t)

	w := postJSONAuth(r, "/auth/login", `{"email":"siapa@example.com"}`)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())

	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Errors, "email")
	assert.Contains(t, resp.Errors, "password")
}
