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
	"stoodify_backend/migrations"
)

// setupRouterE2E memasang DB sqlite in-memory pada config.DB lalu membangun
// router penuh (middleware + validator + rate limit) tanpa menyentuh DB asli.
func setupRouterE2E(t *testing.T) *gin.Engine {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&migrations.User{}, &migrations.UserPreference{}))

	sebelum := config.DB
	config.DB = db
	t.Cleanup(func() { config.DB = sebelum })

	require.NotNil(t, SetupRoutes())
	return SetupRoutes()
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// End-to-end: backend adalah acuan. Payload tanpa school/grade/major ditolak.
func TestE2ERegisterWajibSchoolGradeMajor(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	r := setupRouterE2E(t)

	w := postJSON(r, "/api/auth/register", `{
		"name":"Aliezzar Wijaya",
		"email":"aliezzar42@gmail.com",
		"password":"rahasia123",
		"confirm_password":"rahasia123"
	}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "wajib diisi", resp.Errors["school"])
	assert.Equal(t, "wajib diisi", resp.Errors["grade"])
	assert.Equal(t, "wajib diisi", resp.Errors["major"])
}

// End-to-end: payload lengkap sesuai kontrak berhasil.
func TestE2ERegisterLengkap(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	r := setupRouterE2E(t)

	w := postJSON(r, "/api/auth/register", `{
		"name":"Aliezzar Wijaya",
		"email":"aliezzar42@gmail.com",
		"school":"SMK Telkom Sidoarjo",
		"grade":10,
		"major":"SIJA",
		"password":"rahasia123",
		"confirm_password":"rahasia123"
	}`)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// End-to-end: error 400 di production tidak boleh membocorkan detail Go.
func TestE2EValidasiProductionTanpaBocoran(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	r := setupRouterE2E(t)

	w := postJSON(r, "/api/auth/register", `{"email":"bukan-email","password":"123"}`)

	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	mentah := w.Body.String()
	for _, terlarang := range []string{"RegisterInput", "validation for", "failed on the", "Error:Field", "binding:"} {
		assert.NotContains(t, mentah, terlarang, "response production membocorkan detail internal: %s", mentah)
	}

	var resp struct {
		Errors map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "wajib diisi", resp.Errors["name"])
	assert.Equal(t, "format email tidak valid", resp.Errors["email"])
	assert.Equal(t, "minimal 8 karakter", resp.Errors["password"])
	assert.Equal(t, "wajib diisi", resp.Errors["confirm_password"])
}

// End-to-end: development tetap menyertakan error mentah untuk debugging.
func TestE2EValidasiDevelopmentMenampilkanDetail(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	r := setupRouterE2E(t)

	w := postJSON(r, "/api/auth/register", `{"email":"bukan-email"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "failed on the", "development harus tetap menampilkan detail mentah")
}
