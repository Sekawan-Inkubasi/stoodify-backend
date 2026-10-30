package middlewares

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"stoodify_backend/internal/ratelimit"
	"stoodify_backend/internal/response"
)

// RateLimiter mengembalikan middleware Gin untuk membatasi request per IP per bucket
func RateLimiter(store ratelimit.LimiterStore, limit int, window time.Duration, bucket string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Preflight CORS selalu dilewatkan tanpa memotong kuota
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		key := fmt.Sprintf("%s:%s", bucket, clientIP)

		allowed, remaining, retryAfter := store.Allow(key, limit, window)
		if !allowed {
			waitSeconds := int(retryAfter.Seconds())
			if waitSeconds <= 0 {
				waitSeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(waitSeconds))
			c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Header("X-RateLimit-Remaining", "0")
			response.TooManyRequests(c, "Terlalu banyak permintaan. Silakan coba beberapa saat lagi.")
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Next()
	}
}
