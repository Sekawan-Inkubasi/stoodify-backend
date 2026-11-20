package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stoodify_backend/internal/googleauth"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

const clientIDUji = "1043856692765-q1n1p9ueutn3a7iif5gm3s7081o9koud.apps.googleusercontent.com"

func ptrStr(s string) *string { return &s }

type keyUji struct{ key *rsa.PrivateKey }

func (k keyUji) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	return &k.key.PublicKey, nil
}

type klaimGoogle struct {
	Sub   string
	Email string
	Name  string
}

func setupGoogle(t *testing.T) (*gin.Engine, *rsa.PrivateKey, migrations.User) {
	t.Helper()
	db := setupAITestDB(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	auth := NewAuthController(db)
	auth.Google = &googleauth.Verifier{
		ClientID: clientIDUji,
		Keys:     keyUji{key: key},
		Now:      time.Now,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/google", auth.LoginGoogle)

	user := migrations.User{Name: "Siswa", Email: "siswa@example.com", PasswordHash: "hash", TokenVersion: 1,
		AuthProvider: migrations.AuthProviderPassword, ProfileCompleted: true}
	require.NoError(t, db.Create(&user).Error)
	return r, key, user
}

func tokenGoogle(t *testing.T, key *rsa.PrivateKey, k klaimGoogle) string {
	t.Helper()
	now := time.Now()
	klaim := jwt.MapClaims{
		"iss":            "https://accounts.google.com",
		"aud":            clientIDUji,
		"sub":            k.Sub,
		"email":          k.Email,
		"email_verified": true,
		"name":           k.Name,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, klaim)
	token.Header["kid"] = "kunci-uji"
	raw, err := token.SignedString(key)
	require.NoError(t, err)
	return raw
}

func panggilGoogle(r *gin.Engine, idToken string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"id_token": idToken})
	req := httptest.NewRequest(http.MethodPost, "/auth/google", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLoginGoogleMembuatAkunBaruBelumOnboarding(t *testing.T) {
	r, key, _ := setupGoogle(t)

	w := panggilGoogle(r, tokenGoogle(t, key, klaimGoogle{
		Sub: "sub-baru", Email: "baru@example.com", Name: "Siswa Baru",
	}))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			AccessToken     string `json:"access_token"`
			NeedsOnboarding bool   `json:"needs_onboarding"`
			AuthProvider    string `json:"auth_provider"`
			User            struct {
				Email            string `json:"email"`
				ProfileCompleted bool   `json:"profile_completed"`
			} `json:"user"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Data.AccessToken, "token aplikasi harus langsung diterbitkan")
	assert.True(t, resp.Data.NeedsOnboarding)
	assert.Equal(t, migrations.AuthProviderGoogle, resp.Data.AuthProvider)
	assert.Equal(t, "baru@example.com", resp.Data.User.Email)
	assert.False(t, resp.Data.User.ProfileCompleted)

	var user migrations.User
	require.NoError(t, configDB().Where("google_sub = ?", "sub-baru").First(&user).Error)
	assert.Empty(t, user.PasswordHash, "akun Google tidak punya password")
	assert.Zero(t, user.Grade)
}

func TestLoginGoogleAutoLinkEmailTerdaftar(t *testing.T) {
	r, key, user := setupGoogle(t)

	w := panggilGoogle(r, tokenGoogle(t, key, klaimGoogle{
		Sub: "sub-link", Email: user.Email, Name: "Siswa Lama",
	}))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.User
	require.NoError(t, configDB().First(&sesudah, user.ID).Error)
	require.NotNil(t, sesudah.GoogleSub)
	assert.Equal(t, "sub-link", *sesudah.GoogleSub, "email terverifikasi di-auto-link")
	assert.Equal(t, migrations.AuthProviderPassword, sesudah.AuthProvider, "akun password tetap punya password")
	assert.True(t, sesudah.ProfileCompleted)

	var jumlah int64
	require.NoError(t, configDB().Model(&migrations.User{}).Where("email = ?", user.Email).Count(&jumlah).Error)
	assert.Equal(t, int64(1), jumlah, "tidak boleh membuat akun duplikat")
}

func TestLoginGoogleMenolakEmailTerhubungAkunGoogleLain(t *testing.T) {
	r, key, user := setupGoogle(t)

	terhubung := user
	subLain := "sub-lain"
	terhubung.GoogleSub = &subLain
	require.NoError(t, configDB().Save(&terhubung).Error)

	w := panggilGoogle(r, tokenGoogle(t, key, klaimGoogle{
		Sub: "sub-baru-lain", Email: user.Email, Name: "Penyusup",
	}))

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "sudah terhubung ke akun Google lain")
}

func TestLoginGoogleTokenPalsuDitolak(t *testing.T) {
	r, _, _ := setupGoogle(t)

	w := panggilGoogle(r, "token-ngawur")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NotContains(t, w.Body.String(), "signature", "alasan teknis tidak boleh bocor")
}

func TestLoginPasswordDitolakUntukAkunGoogle(t *testing.T) {
	db := setupAITestDB(t)
	user := migrations.User{
		Name: "Akun Google", Email: "google@example.com", PasswordHash: "",
		GoogleSub: ptrStr("sub-google"), AuthProvider: migrations.AuthProviderGoogle,
		ProfileCompleted: true, TokenVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&migrations.User{}).Where("id = ?", user.ID).Update("profile_completed", false).Error)
	user.ProfileCompleted = false

	gin.SetMode(gin.TestMode)
	auth := NewAuthController(db)
	r := gin.New()
	r.POST("/auth/login", auth.Login)

	body := `{"email":"google@example.com","password":"apa-saja"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "terdaftar lewat Google")
}

func TestOnboardingMelengkapiProfil(t *testing.T) {
	db := setupAITestDB(t)
	user := migrations.User{
		Name: "Akun Google", Email: "onboard@example.com",
		GoogleSub: ptrStr("sub-onboard"), AuthProvider: migrations.AuthProviderGoogle,
		ProfileCompleted: false, TokenVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&migrations.User{}).Where("id = ?", user.ID).Update("profile_completed", false).Error)
	user.ProfileCompleted = false

	gin.SetMode(gin.TestMode)
	ctrl := NewUserController(db)
	r := gin.New()
	r.Use(func(k *gin.Context) {
		k.Set(middlewares.ContextUserKey, user)
		k.Next()
	})
	r.PATCH("/users/me/onboarding", ctrl.CompleteOnboarding)

	body := `{"school":"SMK Telkom Sidoarjo","grade":10,"major":"SIJA"}`
	req := httptest.NewRequest(http.MethodPatch, "/users/me/onboarding", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.User
	require.NoError(t, db.First(&sesudah, user.ID).Error)
	assert.True(t, sesudah.ProfileCompleted)
	assert.Equal(t, "SMK Telkom Sidoarjo", sesudah.School)
	assert.Equal(t, int8(10), sesudah.Grade)
	assert.Equal(t, "SIJA", sesudah.Major)
}

func TestOnboardingMenolakGradeDiLuarRentang(t *testing.T) {
	db := setupAITestDB(t)
	user := migrations.User{
		Name: "Akun Google", Email: "onboard2@example.com",
		GoogleSub: ptrStr("sub-onboard2"), AuthProvider: migrations.AuthProviderGoogle,
		ProfileCompleted: false, TokenVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&migrations.User{}).Where("id = ?", user.ID).Update("profile_completed", false).Error)
	user.ProfileCompleted = false

	gin.SetMode(gin.TestMode)
	ctrl := NewUserController(db)
	r := gin.New()
	r.Use(func(k *gin.Context) {
		k.Set(middlewares.ContextUserKey, user)
		k.Next()
	})
	r.PATCH("/users/me/onboarding", ctrl.CompleteOnboarding)

	body := `{"school":"SMK","grade":3,"major":"SIJA"}`
	req := httptest.NewRequest(http.MethodPatch, "/users/me/onboarding", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var sesudah migrations.User
	require.NoError(t, db.First(&sesudah, user.ID).Error)
	assert.False(t, sesudah.ProfileCompleted, "profil tidak boleh dianggap lengkap")
}

// Regresi: kolom google_sub harus NULL (bukan string kosong) untuk akun non-Google,
// karena index unik menolak banyak nilai "" — dulu ini membuat INSERT user kedua gagal.
func TestGoogleSubUnikTapiBolehNullBanyak(t *testing.T) {
	db := setupAITestDB(t)

	a := migrations.User{Name: "A", Email: "a@example.com", PasswordHash: "x", TokenVersion: 1}
	b := migrations.User{Name: "B", Email: "b@example.com", PasswordHash: "x", TokenVersion: 1}
	require.NoError(t, db.Create(&a).Error, "user tanpa Google harus bisa dibuat berkali-kali")
	require.NoError(t, db.Create(&b).Error)

	sub := "sub-sama"
	c := migrations.User{Name: "C", Email: "c@example.com", PasswordHash: "x", TokenVersion: 1, GoogleSub: &sub}
	require.NoError(t, db.Create(&c).Error)

	d := migrations.User{Name: "D", Email: "d@example.com", PasswordHash: "x", TokenVersion: 1, GoogleSub: &sub}
	assert.Error(t, db.Create(&d).Error, "google_sub yang sama harus ditolak index unik")
}

func TestLoginEndpointMenerimaGoogleIDToken(t *testing.T) {
	db := setupAITestDB(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	auth := NewAuthController(db)
	auth.Google = &googleauth.Verifier{
		ClientID: clientIDUji,
		Keys:     keyUji{key: key},
		Now:      time.Now,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/login", auth.Login)

	idToken := tokenGoogle(t, key, klaimGoogle{
		Sub:   "sub-login-polymorphic",
		Email: "poly@example.com",
		Name:  "Poly User",
	})

	body, _ := json.Marshal(map[string]string{"id_token": idToken})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	assert.NotEmpty(t, payload["data"].(map[string]any)["access_token"])

	var saved migrations.User
	require.NoError(t, db.Where("google_sub = ?", "sub-login-polymorphic").First(&saved).Error)
	assert.Equal(t, "poly@example.com", saved.Email)
}
