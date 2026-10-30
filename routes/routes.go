package routes

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"stoodify_backend/config"
	ctrl "stoodify_backend/controllers"
	"stoodify_backend/docs"
	"stoodify_backend/internal/ratelimit"
	"stoodify_backend/middlewares"
)

const scalarHTML = `<!doctype html>
<html lang="en">
  <head>
    <title>Stoodify API Documentation</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🚀</text></svg>">
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/openapi.yaml"
      data-configuration='{"theme":"purple","layout":"modern","hideModels":false,"searchHotKey":"k"}'></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`

// ConfigureTrustedProxies mengatur trusted proxies pada engine Gin secara aman.
// Jika env kosong atau nilai tidak valid, r akan disetel ke nil (tidak mempercayai header proxy).
func ConfigureTrustedProxies(r *gin.Engine, trustedProxiesEnv string) error {
	if trustedProxiesEnv == "" {
		return r.SetTrustedProxies(nil)
	}

	proxies := strings.Split(trustedProxiesEnv, ",")
	for i := range proxies {
		proxies[i] = strings.TrimSpace(proxies[i])
	}

	if err := r.SetTrustedProxies(proxies); err != nil {
		// Fail-safe: pastikan fallback ke nil agar tidak membiarkan default Gin (trust all) aktif
		_ = r.SetTrustedProxies(nil)
		return fmt.Errorf("konfigurasi TRUSTED_PROXIES tidak valid: %w", err)
	}
	return nil
}

func SetupRoutes() *gin.Engine {
	r := gin.Default()

	// Batasi proxy tepercaya: hanya percaya header XFF jika IP proxy dikonfigurasi dengan benar
	if err := ConfigureTrustedProxies(r, os.Getenv("TRUSTED_PROXIES")); err != nil {
		log.Printf("[SECURITY WARNING] %v; fallback ke SetTrustedProxies(nil)", err)
	}

	r.Use(middlewares.CORS())

	// Rate limiter in-memory store
	limiterStore := ratelimit.NewMemoryStore(2*time.Minute, 50000)

	// OpenAPI Spec & Scalar Interactive Documentation
	openapiHandler := func(c *gin.Context) {
		if c.Request.Method == http.MethodHead {
			c.Header("Content-Type", "application/yaml; charset=utf-8")
			c.Status(http.StatusOK)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPISpec)
	}
	r.GET("/openapi.yaml", openapiHandler)
	r.HEAD("/openapi.yaml", openapiHandler)

	docsHandler := func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		if c.Request.Method == http.MethodHead {
			c.Status(http.StatusOK)
			return
		}
		c.String(http.StatusOK, scalarHTML)
	}
	r.GET("/docs", docsHandler)
	r.HEAD("/docs", docsHandler)

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Stoodify AI Assistant Backend API",
			"data": gin.H{
				"status":     "online",
				"version":    "1.0.0",
				"api_base":   "/api",
				"docs":       "/docs",
				"repository": "https://github.com/Sekawan-Inkubasi/stoodify-backend",
			},
		})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "stoodify-backend",
		})
	})

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Endpoint tidak ditemukan. Semua endpoint API berada di bawah prefix /api",
		})
	})

	// Inisialisasi Controllers (Inject DB)
	authCtrl := ctrl.NewAuthController(config.DB)
	userCtrl := ctrl.NewUserController(config.DB)
	subjectCtrl := ctrl.NewSubjectController(config.DB)
	scheduleCtrl := ctrl.NewScheduleController(config.DB)
	taskCtrl := ctrl.NewTaskController(config.DB)
	subtaskCtrl := ctrl.NewSubtaskController(config.DB)
	activityCtrl := ctrl.NewActivityController(config.DB)
	aiCtrl := ctrl.NewAIController(config.DB)
	notificationCtrl := ctrl.NewNotificationController(config.DB)
	sessionCtrl := ctrl.NewStudySessionController(config.DB)
	statsCtrl := ctrl.NewStatisticsController(config.DB)
	curriculumCtrl := ctrl.NewCurriculumController(config.DB)

	// Base API Group (Limit umum: 60 req/menit)
	api := r.Group("/api")
	api.Use(middlewares.RateLimiter(limiterStore, 60, time.Minute, "api"))
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "Welcome to Stoodify API",
				"data": gin.H{
					"service": "stoodify-backend",
					"status":  "healthy",
					"endpoints": gin.H{
						"auth":           "/api/auth",
						"users":          "/api/users",
						"subjects":       "/api/subjects",
						"schedules":      "/api/schedules",
						"tasks":          "/api/tasks",
						"activities":     "/api/activities",
						"ai":             "/api/ai",
						"notifications":  "/api/notifications",
						"study_sessions": "/api/study-sessions",
						"statistics":     "/api/statistics",
						"curriculum":     "/api/curriculum",
					},
				},
			})
		})

		// ==========================================
		// 1. AUTH GROUP (Sebagian Public, Sebagian Protected)
		// ==========================================
		authLimiter := middlewares.RateLimiter(limiterStore, 10, time.Minute, "auth")
		auth := api.Group("/auth")
		{
			// Public (Strict rate limit: 10 req/menit)
			auth.POST("/register", authLimiter, authCtrl.Register)
			auth.POST("/login", authLimiter, authCtrl.Login)
			auth.POST("/refresh", authLimiter, authCtrl.Refresh) // Menerima refresh_token di body
			auth.POST("/password-reset/request", authLimiter, authCtrl.PasswordResetRequest)
			auth.POST("/password-reset/confirm", authLimiter, authCtrl.PasswordResetConfirm)

			// Protected (Butuh Access Token valid)
			authProtected := auth.Group("")
			authProtected.Use(middlewares.RequireAuth())
			{
				authProtected.GET("/me", authCtrl.Me)
				authProtected.POST("/logout", authCtrl.Logout)
				authProtected.POST("/password/change", authCtrl.ChangePassword)
			}
		}

		// ==========================================
		// 2. PROTECTED GROUPS (Semua butuh Access Token)
		// ==========================================
		protected := api.Group("")
		protected.Use(middlewares.RequireAuth())
		{
			// --- USERS ---
			users := protected.Group("/users")
			{
				users.GET("/me", userCtrl.GetMe)
				users.PATCH("/me", userCtrl.UpdateMe)
				users.POST("/change-password", authCtrl.ChangePassword)
				users.GET("/me/preferences", userCtrl.GetPreferences)
				users.PATCH("/me/preferences", userCtrl.UpdatePreferences)
			}

			// --- SUBJECTS ---
			subjects := protected.Group("/subjects")
			{
				subjects.GET("/", subjectCtrl.GetAll)
				subjects.POST("/", subjectCtrl.Create)
				subjects.GET("/:id", subjectCtrl.GetByID)
				subjects.PATCH("/:id", subjectCtrl.Update)
				subjects.DELETE("/:id", subjectCtrl.Delete)
			}

			// --- SCHEDULES ---
			schedules := protected.Group("/schedules")
			{
				schedules.GET("/", scheduleCtrl.GetAll)
				schedules.POST("/", scheduleCtrl.Create)
				schedules.GET("/today", scheduleCtrl.GetToday)
				schedules.GET("/week", scheduleCtrl.GetWeek)
				schedules.PATCH("/:id", scheduleCtrl.Update)
				schedules.DELETE("/:id", scheduleCtrl.Delete)
			}

			// --- TASKS ---
			tasks := protected.Group("/tasks")
			{
				tasks.GET("/", taskCtrl.GetAll)
				tasks.POST("/", taskCtrl.Create)
				tasks.GET("/upcoming", taskCtrl.GetUpcoming)
				tasks.GET("/overdue", taskCtrl.GetOverdue)
				tasks.GET("/:id", taskCtrl.GetByID)
				tasks.PATCH("/:id", taskCtrl.Update)
				tasks.DELETE("/:id", taskCtrl.Delete)
				tasks.PATCH("/:id/status", taskCtrl.UpdateStatus)
				tasks.PATCH("/:id/progress", taskCtrl.UpdateProgress)
			}

			// --- SUBTASKS ---
			// Nested under tasks; pakai :id agar tidak bentrok dengan /tasks/:id
			subtasksNested := protected.Group("/tasks/:id/subtasks")
			{
				subtasksNested.GET("/", subtaskCtrl.GetAllByTask)
				subtasksNested.POST("/", subtaskCtrl.Create)
			}
			// Single subtask operations
			subtasksSingle := protected.Group("/subtasks")
			{
				subtasksSingle.PATCH("/:id", subtaskCtrl.Update)
				subtasksSingle.DELETE("/:id", subtaskCtrl.Delete)
			}

			// --- ACTIVITIES ---
			activities := protected.Group("/activities")
			{
				activities.GET("/", activityCtrl.GetAll)
				activities.POST("/", activityCtrl.Create)
				activities.PATCH("/:id", activityCtrl.Update)
				activities.DELETE("/:id", activityCtrl.Delete)
			}

			// --- AI BRIDGE ---
			ai := protected.Group("/ai")
			{
				// Schedule Recommendations
				ai.POST("/schedule-recommendations/generate", aiCtrl.GenerateSchedule)
				ai.GET("/schedule-recommendations", aiCtrl.GetScheduleRecommendations)
				ai.POST("/schedule-recommendations/:id/accept", aiCtrl.AcceptRecommendation)
				ai.POST("/schedule-recommendations/:id/reject", aiCtrl.RejectRecommendation)
				ai.POST("/reschedule", aiCtrl.Reschedule)

				// Material Predictions
				ai.POST("/material-predictions/generate", aiCtrl.GeneratePrediction)
				ai.GET("/material-predictions/:subjectId", aiCtrl.GetPrediction)
				ai.POST("/material-predictions/:id/feedback", aiCtrl.SubmitFeedback)
			}

			// --- NOTIFICATIONS ---
			notifications := protected.Group("/notifications")
			{
				notifications.GET("/", notificationCtrl.GetAll)
				notifications.PATCH("/:id/read", notificationCtrl.MarkAsRead)
				notifications.PATCH("/read-all", notificationCtrl.MarkAllAsRead)
			}

			// --- STUDY SESSIONS ---
			sessions := protected.Group("/study-sessions")
			{
				sessions.GET("/", sessionCtrl.GetAll)
				sessions.POST("/", sessionCtrl.Create)
				sessions.GET("/today", sessionCtrl.GetToday)
				sessions.PATCH("/:id", sessionCtrl.Update)
				sessions.DELETE("/:id", sessionCtrl.Delete)
			}

			// --- STATISTICS ---
			statistics := protected.Group("/statistics")
			{
				statistics.GET("/overview", statsCtrl.GetOverview)
				statistics.GET("/weekly", statsCtrl.GetWeekly)
				statistics.GET("/monthly", statsCtrl.GetMonthly)
				statistics.GET("/subjects", statsCtrl.GetBySubjects)
			}

			// --- CURRICULUM & SYLLABUS (ROADMAP) ---
			curriculum := protected.Group("/curriculum")
			{
				curriculum.GET("/roadmap/:subjectId", curriculumCtrl.GetRoadmapBySubject)
				curriculum.POST("/topics", curriculumCtrl.CreateTopic)
				curriculum.POST("/topics/:topicId/progress", curriculumCtrl.UpdateProgress)
			}
		}
	}

	return r
}
