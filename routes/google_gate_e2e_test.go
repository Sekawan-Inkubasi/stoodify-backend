package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stoodify_backend/config"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

func ptrSub(s string) *string { return &s }

// setupRouterOAuth menyiapkan router penuh + DB sqlite dengan seluruh tabel,
// lalu mengembalikan akun Google yang belum onboarding beserta access token-nya.
func setupRouterOAuth(t *testing.T) (*gin.Engine, migrations.User, string) {
	t.Helper()
	t.Setenv("JWT_SECRET", "rahasia-uji-jwt")
	t.Setenv("APP_ENV", "production")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(migrations.AllModels()...))

	sebelum := config.DB
	config.DB = db
	t.Cleanup(func() { config.DB = sebelum })

	user := migrations.User{
		Name: "Akun Google", Email: "oauth@example.com",
		GoogleSub: ptrSub("sub-oauth"), AuthProvider: migrations.AuthProviderGoogle,
		TokenVersion: 1,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Model(&migrations.User{}).Where("id = ?", user.ID).Update("profile_completed", false).Error)
	user.ProfileCompleted = false

	access, err := token.GenerateAccess(user.ID, user.TokenVersion)
	require.NoError(t, err)

	return SetupRoutes(), user, access.Token
}

func doAuth(r *gin.Engine, method, path, body, accessToken string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestE2EOnboardingMembukaFiturAI(t *testing.T) {
	r, user, access := setupRouterOAuth(t)

	// 1. Sebelum onboarding: fitur AI ditolak 409 dengan pesan yang bisa ditampilkan.
	w := doAuth(r, http.MethodGet, "/api/ai/schedule-recommendations", "", access)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Lengkapi profil dulu")

	// 2. Lengkapi profil.
	w = doAuth(r, http.MethodPatch, "/api/users/me/onboarding",
		`{"school":"SMK Telkom Sidoarjo","grade":10,"major":"SIJA"}`, access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Data struct {
			ProfileCompleted bool `json:"profile_completed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Data.ProfileCompleted)

	// 3. Setelah onboarding: endpoint terbuka (200, daftar sesi masih kosong).
	w = doAuth(r, http.MethodGet, "/api/ai/schedule-recommendations", "", access)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.User
	require.NoError(t, config.DB.First(&sesudah, user.ID).Error)
	assert.True(t, sesudah.ProfileCompleted)
	assert.Equal(t, "SMK Telkom Sidoarjo", sesudah.School)
}
