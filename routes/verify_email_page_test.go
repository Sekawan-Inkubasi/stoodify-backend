package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stoodify_backend/config"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

func setupHalamanVerifikasi(t *testing.T) (*gin.Engine, migrations.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&migrations.User{}, &migrations.EmailVerificationToken{}))

	sebelum := config.DB
	config.DB = db
	t.Cleanup(func() { config.DB = sebelum })

	user := migrations.User{Name: "Siswa", Email: "page@example.com", PasswordHash: "x", TokenVersion: 1}
	require.NoError(t, db.Create(&user).Error)

	return SetupRoutes(), user
}

func simpanToken(t *testing.T, userID uint, umur time.Duration) string {
	t.Helper()
	raw, hash, err := token.GenerateRandomToken()
	require.NoError(t, err)
	require.NoError(t, config.DB.Create(&migrations.EmailVerificationToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(umur),
	}).Error)
	return raw
}

func getHalaman(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHalamanVerifikasiTokenValidMenawarkanDeepLink(t *testing.T) {
	r, user := setupHalamanVerifikasi(t)
	raw := simpanToken(t, user.ID, time.Hour)

	w := getHalaman(r, "/verify-email?token="+raw)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "Buka Aplikasi Stoodify")
	assert.Contains(t, body, "stoodify://verify-email?token="+raw)
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
}

// Halaman tidak boleh "menghabiskan" token: email client sering prefetch link.
func TestHalamanVerifikasiTidakMenandaiTokenTerpakai(t *testing.T) {
	r, user := setupHalamanVerifikasi(t)
	raw := simpanToken(t, user.ID, time.Hour)

	require.Equal(t, http.StatusOK, getHalaman(r, "/verify-email?token="+raw).Code)

	var record migrations.EmailVerificationToken
	require.NoError(t, config.DB.Where("user_id = ?", user.ID).First(&record).Error)
	assert.Nil(t, record.UsedAt, "membuka halaman tidak boleh menandai token terpakai")
}

func TestHalamanVerifikasiTokenKedaluwarsa(t *testing.T) {
	r, user := setupHalamanVerifikasi(t)
	raw := simpanToken(t, user.ID, -time.Minute)

	w := getHalaman(r, "/verify-email?token="+raw)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "kedaluwarsa")
	assert.NotContains(t, body, "stoodify://", "token kedaluwarsa tidak boleh membuka deep link")
}

func TestHalamanVerifikasiTokenSudahDipakai(t *testing.T) {
	r, user := setupHalamanVerifikasi(t)
	raw := simpanToken(t, user.ID, time.Hour)
	require.NoError(t, config.DB.Model(&migrations.EmailVerificationToken{}).
		Where("user_id = ?", user.ID).Update("used_at", time.Now()).Error)

	w := getHalaman(r, "/verify-email?token="+raw)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sudah diverifikasi")
}

func TestHalamanVerifikasiTokenAsing(t *testing.T) {
	r, _ := setupHalamanVerifikasi(t)

	w := getHalaman(r, "/verify-email?token=token-ngawur")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tidak valid")
}

func TestHalamanVerifikasiTanpaToken(t *testing.T) {
	r, _ := setupHalamanVerifikasi(t)
	w := getHalaman(r, "/verify-email")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Link tidak lengkap")
}

// Token berbahaya tidak boleh bisa menyuntik HTML/JS ke halaman.
func TestHalamanVerifikasiAmanDariInjeksiQuery(t *testing.T) {
	r, _ := setupHalamanVerifikasi(t)

	w := getHalaman(r, `/verify-email?token="><script>alert(1)</script>`)

	body := w.Body.String()
	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.False(t, strings.Contains(body, `location.href=""><`), "deep link hanya dibuat untuk token valid")
}
