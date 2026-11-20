package middlewares

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseAllowedOrigins membaca daftar origin tepercaya dari env CORS_ALLOWED_ORIGINS
// (dipisahkan koma). Kosong = tidak ada origin browser yang diberi izin kredensial.
func parseAllowedOrigins(raw string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		if o := strings.TrimSpace(item); o != "" {
			allowed[o] = struct{}{}
		}
	}
	return allowed
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.Request.Header.Get("Origin"))
		allowed := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))

		if len(allowed) > 0 {
			if origin != "" {
				if _, ok := allowed[origin]; ok {
					c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					c.Writer.Header().Set("Vary", "Origin")
					c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
		} else {
			// Default aman saat CORS_ALLOWED_ORIGINS kosong: jangan pernah memantulkan
			// Origin sembarang bersama Access-Control-Allow-Credentials: true.
			// Client native mobile tidak memakai browser CORS; wildcard tanpa credentials
			// tetap memungkinkan akses non-kredensial (mis. Scalar /docs).
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
