package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"stoodify_backend/migrations"
)

func routerGate(user migrations.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(k *gin.Context) {
		k.Set(ContextUserKey, user)
		k.Next()
	})
	r.GET("/butuh-profil", RequireProfileComplete(), func(k *gin.Context) {
		k.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestRequireProfileCompleteMenolak409(t *testing.T) {
	user := migrations.User{ID: 1, Email: "google@example.com", AuthProvider: migrations.AuthProviderGoogle, ProfileCompleted: false}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/butuh-profil", nil)
	routerGate(user).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Lengkapi profil dulu")
	assert.Contains(t, w.Body.String(), `"profile_completed":"false"`)
}

func TestRequireProfileCompleteMeloloskanProfilLengkap(t *testing.T) {
	user := migrations.User{ID: 1, Email: "siswa@example.com", AuthProvider: migrations.AuthProviderPassword, ProfileCompleted: true}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/butuh-profil", nil)
	routerGate(user).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
