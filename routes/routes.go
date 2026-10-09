package routes

import (
	"github.com/gin-gonic/gin"

	"stoodify_backend/config"
	ctrl "stoodify_backend/controllers"
	"stoodify_backend/middlewares"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()

	authCtrl := ctrl.NewAuthController(config.DB)

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		auth.POST("/register", authCtrl.Register)
		auth.POST("/login", authCtrl.Login)

		session := auth.Group("")
		session.Use(middlewares.RequireAuth(config.DB))
		session.GET("/me", authCtrl.Me)
	}
	return r
}
