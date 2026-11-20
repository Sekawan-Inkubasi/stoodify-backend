package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerInput meniru DTO registrasi yang memicu keluhan di mobile.
type registerInput struct {
	Name            string `json:"name" binding:"required,min=1,max=100"`
	Email           string `json:"email" binding:"required,email"`
	School          string `json:"school" binding:"required,max=150"`
	Grade           int8   `json:"grade" binding:"required,min=10,max=13"`
	Major           string `json:"major" binding:"required,max=100"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

func validasi(t *testing.T, payload string) error {
	t.Helper()
	v := validator.New()
	v.SetTagName("binding") // sama seperti validator milik Gin
	DaftarkanNamaFieldJSON(v)

	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &raw))

	input := registerInput{
		Name:            str(raw["name"]),
		Email:           str(raw["email"]),
		School:          str(raw["school"]),
		Grade:           int8(num(raw["grade"])),
		Major:           str(raw["major"]),
		Password:        str(raw["password"]),
		ConfirmPassword: str(raw["confirm_password"]),
	}
	return v.Struct(input)
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func num(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func jalankanValidationError(t *testing.T, payload string) ErrorResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)

	err := validasi(t, payload)
	require.Error(t, err)
	ValidationError(c, err)

	require.Equal(t, http.StatusBadRequest, w.Code)
	return decodeError(t, w)
}

func TestValidationErrorDevelopmentMenampilkanDetailMentah(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	body := jalankanValidationError(t, `{"name":"Aliezzar","email":"a@b.com","password":"rahasia123","confirm_password":"rahasia123"}`)

	// Detail mentah hanya boleh muncul di non-production.
	assert.Contains(t, body.Errors["body"], "registerInput.school")
	assert.Contains(t, body.Errors["body"], "failed on the 'required' tag")
	assert.Equal(t, "wajib diisi", body.Errors["school"])
	assert.Equal(t, "wajib diisi", body.Errors["grade"])
	assert.Equal(t, "wajib diisi", body.Errors["major"])
	// Field yang valid tidak boleh muncul.
	_, ada := body.Errors["name"]
	assert.False(t, ada)
}

func TestValidationErrorProductionMenyembunyikanDetailInternal(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	body := jalankanValidationError(t, `{"name":"Aliezzar","email":"a@b.com","password":"rahasia123","confirm_password":"rahasia123"}`)

	assert.Equal(t, "Data yang dikirim belum lengkap atau tidak valid", body.Message)

	// Tidak boleh ada jejak internal: nama struct, tag validator, path file.
	mentah, err := json.Marshal(body)
	require.NoError(t, err)
	for _, terlarang := range []string{"RegisterInput", "validation for", "failed on the", "Error:Field", "validator"} {
		assert.NotContains(t, string(mentah), terlarang, "response production membocorkan detail internal")
	}

	// Pesan per field tetap ada supaya aplikasi bisa menandai kolomnya.
	assert.Equal(t, "wajib diisi", body.Errors["school"])
	assert.Equal(t, "wajib diisi", body.Errors["grade"])
	assert.Equal(t, "wajib diisi", body.Errors["major"])
	assert.NotContains(t, body.Errors, "body")
}

func TestValidationErrorPesanRamahPerTag(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	body := jalankanValidationError(t, `{"name":"","email":"bukan-email","school":"SMA","grade":9,"major":"IPA","password":"123","confirm_password":""}`)

	// Tag pertama yang gagal dilaporkan (required menang atas min).
	assert.Equal(t, "wajib diisi", body.Errors["name"])
	assert.Equal(t, "format email tidak valid", body.Errors["email"])
	assert.Equal(t, "minimal 10", body.Errors["grade"])
	assert.Equal(t, "minimal 8 karakter", body.Errors["password"])
	assert.Equal(t, "wajib diisi", body.Errors["confirm_password"])
}

func TestValidationErrorBukanValidatorTetapAmanDiProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/register", nil)

	ValidationError(c, errors.New("unexpected end of JSON input"))

	require.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeError(t, w)
	assert.NotContains(t, body.Errors["body"], "JSON")
	assert.Equal(t, "data tidak valid", body.Errors["body"])
}
