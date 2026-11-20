package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS_DefaultNoArbitraryOriginWithCredentials(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"),
		"origin sembarang tidak boleh dipantulkan dengan Access-Control-Allow-Credentials")
}

func TestCORS_AllowlistOnlyGrantsCredentialsToTrustedOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.stoodify.id, http://localhost:3000")
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	// Origin tepercaya -> diizinkan + credentials + Vary: Origin
	reqOK := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqOK.Header.Set("Origin", "https://app.stoodify.id")
	wOK := httptest.NewRecorder()
	r.ServeHTTP(wOK, reqOK)

	assert.Equal(t, "https://app.stoodify.id", wOK.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", wOK.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "Origin", wOK.Header().Get("Vary"))

	// Origin asing -> tidak mendapat header ACAO / ACAC
	reqEvil := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqEvil.Header.Set("Origin", "https://evil.example.com")
	wEvil := httptest.NewRecorder()
	r.ServeHTTP(wEvil, reqEvil)

	assert.Empty(t, wEvil.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, wEvil.Header().Get("Access-Control-Allow-Credentials"))
}
