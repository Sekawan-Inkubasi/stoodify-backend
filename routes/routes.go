package routes

import (
	"github.com/gin-gonic/gin"

	"stoodify_backend/config"
	ctrl "stoodify_backend/controllers"
	"stoodify_backend/middlewares"
)

func SetupRoutes() *gin.Engine {
	r := gin.Default()

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

	// Base API Group
	api := r.Group("/api")
	{
		// ==========================================
		// 1. AUTH GROUP (Sebagian Public, Sebagian Protected)
		// ==========================================
		auth := api.Group("/auth")
		{
			// Public (Tidak butuh token)
			auth.POST("/register", authCtrl.Register)
			auth.POST("/login", authCtrl.Login)
			auth.POST("/refresh", authCtrl.Refresh) // Menerima refresh_token di body

			// Protected (Butuh Access Token valid)
			authProtected := auth.Group("")
			authProtected.Use(middlewares.RequireAuth())
			{
				authProtected.GET("/me", authCtrl.Me)
				authProtected.POST("/logout", authCtrl.Logout)
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
		}
	}

	return r
}
