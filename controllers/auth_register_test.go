package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

func routerRegister(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		response.DaftarkanNamaFieldJSON(v)
	}
	r := gin.New()
	auth := NewAuthController(configDB())
	r.POST("/auth/register", auth.Register)
	return r
}

func TestRegisterCukupNamaEmailPassword(t *testing.T) {
	setupAITestDB(t)
	r := routerRegister(t)

	body := `{"name":"Aliezzar Wijaya","email":"aliezzar42@gmail.com","password":"rahasia123","confirm_password":"rahasia123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, "daftar tanpa school/grade/major harus berhasil: %s", w.Body.String())

	var user migrations.User
	require.NoError(t, configDB().Where("email = ?", "aliezzar42@gmail.com").First(&user).Error)
	assert.Equal(t, "Aliezzar Wijaya", user.Name)
	assert.Empty(t, user.School)
	assert.Zero(t, user.Grade)
}

func TestRegisterMenolakGradeDiLuarRentang(t *testing.T) {
	setupAITestDB(t)
	r := routerRegister(t)

	body := `{"name":"Siswa","email":"siswa@example.com","grade":3,"password":"rahasia123","confirm_password":"rahasia123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp struct {
		Success bool              `json:"success"`
		Errors  map[string]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "minimal 7", resp.Errors["grade"])
}

func TestRegisterErrorTidakMembocorkanNamaStruct(t *testing.T) {
	setupAITestDB(t)
	t.Setenv("APP_ENV", "production")
	r := routerRegister(t)

	body := `{"email":"bukan-email","password":"123","confirm_password":""}`
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	mentah := w.Body.String()
	for _, terlarang := range []string{"RegisterInput", "validation for", "failed on the", "Error:Field"} {
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
