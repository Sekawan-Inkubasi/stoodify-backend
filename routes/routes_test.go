package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureTrustedProxies_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	err := ConfigureTrustedProxies(r, "127.0.0.1, 10.0.0.0/8")
	if err != nil {
		t.Fatalf("expected nil error untuk valid proxies, got: %v", err)
	}
}

func TestConfigureTrustedProxies_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	err := ConfigureTrustedProxies(r, "")
	if err != nil {
		t.Fatalf("expected nil error untuk empty env, got: %v", err)
	}
}

func TestConfigureTrustedProxies_InvalidFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Format CIDR tidak valid
	err := ConfigureTrustedProxies(r, "invalid-ip-or-cidr-999.999.999.999/99")
	if err == nil {
		t.Fatal("expected error untuk invalid proxies, tapi got nil")
	}

	// Pastikan fail-safe aktif: Gin TIDAK mempercayai header XFF dari proxy yang tidak valid
	r.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.168.1.50:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.195")
	r.ServeHTTP(w, req)

	// ClientIP harus 192.168.1.50 (RemoteAddr), bukan 203.0.113.195 (XFF palsu)
	if w.Body.String() != "192.168.1.50" {
		t.Errorf("expected client IP 192.168.1.50 (RemoteAddr), got %s", w.Body.String())
	}
}
