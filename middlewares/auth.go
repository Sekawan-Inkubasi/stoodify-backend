package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

const ContextUserKey = "auth_user"

func RequireAuth(db *gorm.DB) gin.HandlerFunc {
	return func(konteks *gin.Context) {
		bearer, ok := strings.CutPrefix(konteks.GetHeader("Authorization"), "Bearer ")
		if !ok || bearer == "" {
			response.Unauthorized(konteks, "token tidak ditemukan")
			konteks.Abort()
			return
		}

		userID, err := token.Parse(bearer)
		if err != nil {
			response.Unauthorized(konteks, "token tidak valid atau sudah kedaluwarsa")
			konteks.Abort()
			return
		}

		var user migrations.User
		if err := db.WithContext(konteks.Request.Context()).First(&user, userID).Error; err != nil {
			response.Unauthorized(konteks, "user dari token tidak ditemukan")
			konteks.Abort()
			return
		}

		konteks.Set(ContextUserKey, user)
		konteks.Next()
	}
}

func CurrentUser(konteks *gin.Context) (migrations.User, bool) {
	user, ok := konteks.Get(ContextUserKey)
	if !ok {
		return migrations.User{}, false
	}
	casted, ok := user.(migrations.User)
	return casted, ok
}
