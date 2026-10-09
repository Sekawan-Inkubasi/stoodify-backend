package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"

	"stoodify_backend/config"
	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

const ContextUserKey = "auth_user"

func RequireAuth() gin.HandlerFunc {
	return func(konteks *gin.Context) {
		bearer, ok := strings.CutPrefix(konteks.GetHeader("Authorization"), "Bearer ")
		if !ok || bearer == "" {
			response.Unauthorized(konteks, "token tidak ditemukan")
			konteks.Abort()
			return
		}

		userID, _, err := token.Parse(bearer, token.TypeAccess)
		if err != nil {
			response.Unauthorized(konteks, "token tidak valid atau sudah kedaluwarsa")
			konteks.Abort()
			return
		}

		var user migrations.User
		if err := config.DB.WithContext(konteks.Request.Context()).First(&user, userID).Error; err != nil {
			response.Unauthorized(konteks, "user dari token tidak ditemukan")
			konteks.Abort()
			return
		}

		konteks.Set(ContextUserKey, user)
		konteks.Next()
	}
}

// RequireRole: role-based access. Role diambil dari token (di-set RequireAuth).
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(konteks *gin.Context) {
		user, ok := CurrentUser(konteks)
		if !ok {
			response.Unauthorized(konteks, "token tidak valid")
			konteks.Abort()
			return
		}
		if !allowed[user.Role] {
			response.Forbidden(konteks, "role tidak diizinkan mengakses endpoint ini")
			konteks.Abort()
			return
		}
		konteks.Next()
	}
}

func CurrentUser(konteks *gin.Context) (migrations.User, bool) {
	raw, ok := konteks.Get(ContextUserKey)
	if !ok {
		return migrations.User{}, false
	}
	user, ok := raw.(migrations.User)
	return user, ok
}

// MustUser: dipakai handler yang sudah pasti lewat RequireAuth.
func MustUser(konteks *gin.Context) migrations.User {
	user, _ := CurrentUser(konteks)
	return user
}
