package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"stoodify_backend/internal/ratelimit"
	"stoodify_backend/internal/response"
)

func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := ratelimit.NewMemoryStore(1*time.Minute, 100)
	defer store.Close()

	r := gin.New()
	r.Use(RateLimiter(store, 2, 1*time.Second, "test-bucket"))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	r.OPTIONS("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// Request 1
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("request 1 expected 200, got %d", w1.Code)
	}

	// Request 2
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("request 2 expected 200, got %d", w2.Code)
	}

	// Request 3 (Harus 429)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3 expected 429, got %d", w3.Code)
	}
	if w3.Header().Get("Retry-After") == "" {
		t.Errorf("header Retry-After harus ada pada 429")
	}

	var resp response.ErrorResponse
	if err := json.Unmarshal(w3.Body.Bytes(), &resp); err != nil {
		t.Fatalf("gagal parse envelope error: %v", err)
	}
	if resp.Success {
		t.Errorf("expected success=false pada error envelope")
	}

	// Preflight CORS (OPTIONS) harus selalu berhasil meski kuota habis
	wOpt := httptest.NewRecorder()
	reqOpt, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
	reqOpt.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(wOpt, reqOpt)
	if wOpt.Code != http.StatusNoContent {
		t.Errorf("OPTIONS preflight expected 204, got %d", wOpt.Code)
	}
}

func TestRateLimiter_UntrustedProxyXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := ratelimit.NewMemoryStore(1*time.Minute, 100)
	defer store.Close()

	r := gin.New()
	_ = r.SetTrustedProxies(nil) // Default: jangan percaya header proxy
	r.Use(RateLimiter(store, 2, 1*time.Second, "auth-bucket"))
	r.GET("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	remoteIP := "1.2.3.4:5678"

	// Request 1 dengan X-Forwarded-For palsu A
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/login", nil)
	req1.RemoteAddr = remoteIP
	req1.Header.Set("X-Forwarded-For", "203.0.113.19")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("req 1 expected 200, got %d", w1.Code)
	}

	// Request 2 dengan X-Forwarded-For palsu B
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/login", nil)
	req2.RemoteAddr = remoteIP
	req2.Header.Set("X-Forwarded-For", "198.51.100.22")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("req 2 expected 200, got %d", w2.Code)
	}

	// Request 3 dengan X-Forwarded-For palsu C (kuota RemoteAddr sudah habis, harus 429)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/login", nil)
	req3.RemoteAddr = remoteIP
	req3.Header.Set("X-Forwarded-For", "192.0.2.99")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("req 3 expected 429 meski header XFF dirotasi, got %d", w3.Code)
	}
}
