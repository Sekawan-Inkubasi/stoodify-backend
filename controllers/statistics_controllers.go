package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type StatisticsController struct {
	DB *gorm.DB
}

func NewStatisticsController(db *gorm.DB) *StatisticsController {
	return &StatisticsController{DB: db}
}

type overviewStats struct {
	TotalTasks     int64   `json:"total_tasks"`
	DoneTasks      int64   `json:"done_tasks"`
	LateTasks      int64   `json:"late_tasks"`
	PendingTasks   int64   `json:"pending_tasks"`
	OnTimeRate     float64 `json:"on_time_rate"`
	TotalStudyMins int64   `json:"total_study_minutes"`
	TotalSubjects  int64   `json:"total_subjects"`
	StreakDays     int     `json:"streak_days"`
}

func (ctrl *StatisticsController) calculateStreak(ctx *gin.Context, userID uint) int {
	type activityDate struct {
		ActDate string
	}
	var dates []activityDate

	query := `
		SELECT to_char(d, 'YYYY-MM-DD') as act_date FROM (
			SELECT updated_at as d FROM tasks WHERE user_id = ? AND status = 'done'
			UNION
			SELECT session_date as d FROM study_sessions WHERE user_id = ? AND status = 'completed'
		) combined
		GROUP BY act_date
		ORDER BY act_date DESC
		LIMIT 60
	`
	ctrl.DB.WithContext(ctx.Request.Context()).Raw(query, userID, userID).Scan(&dates)

	if len(dates) == 0 {
		return 0
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	if dates[0].ActDate != today && dates[0].ActDate != yesterday {
		return 0
	}

	streak := 1
	currentParsed, _ := time.Parse("2006-01-02", dates[0].ActDate)

	for i := 1; i < len(dates); i++ {
		prevParsed, _ := time.Parse("2006-01-02", dates[i].ActDate)
		diffDays := int(currentParsed.Sub(prevParsed).Hours() / 24)

		if diffDays == 1 {
			streak++
			currentParsed = prevParsed
		} else if diffDays <= 2 && (currentParsed.Weekday() == time.Monday || currentParsed.Weekday() == time.Sunday) {
			streak++
			currentParsed = prevParsed
		} else {
			break
		}
	}

	return streak
}

func (ctrl *StatisticsController) GetOverview(konteks *gin.Context) {
	user := currentUser(konteks)
	ctx := konteks.Request.Context()

	var stats overviewStats
	db := ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).Where("user_id = ?", user.ID)

	db.Count(&stats.TotalTasks)
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).Where("user_id = ? AND status = ?", user.ID, migrations.StatusDone).Count(&stats.DoneTasks)
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).Where("user_id = ? AND status = ?", user.ID, migrations.StatusLate).Count(&stats.LateTasks)
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).Where("user_id = ? AND status = ?", user.ID, migrations.StatusPending).Count(&stats.PendingTasks)
	ctrl.DB.WithContext(ctx).Model(&migrations.Subject{}).Where("user_id = ?", user.ID).Count(&stats.TotalSubjects)

	var studyMins struct {
		Total int64
	}
	ctrl.DB.WithContext(ctx).Model(&migrations.StudySession{}).
		Select("COALESCE(SUM(duration),0) as total").
		Where("user_id = ? AND status = ?", user.ID, migrations.SessionCompleted).
		Scan(&studyMins)
	stats.TotalStudyMins = studyMins.Total

	finished := stats.DoneTasks + stats.LateTasks
	if finished > 0 {
		stats.OnTimeRate = float64(stats.DoneTasks) / float64(finished) * 100
	}

	stats.StreakDays = ctrl.calculateStreak(konteks, user.ID)

	response.OK(konteks, "ringkasan statistik", stats)
}

func (ctrl *StatisticsController) GetWeekly(konteks *gin.Context) {
	ctrl.rangeStats(konteks, time.Now().AddDate(0, 0, -7))
}

func (ctrl *StatisticsController) GetMonthly(konteks *gin.Context) {
	ctrl.rangeStats(konteks, time.Now().AddDate(0, -1, 0))
}

func (ctrl *StatisticsController) rangeStats(konteks *gin.Context, since time.Time) {
	user := currentUser(konteks)
	ctx := konteks.Request.Context()

	var created, done, late int64
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).
		Where("user_id = ? AND created_at >= ?", user.ID, since).Count(&created)
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).
		Where("user_id = ? AND status = ? AND updated_at >= ?", user.ID, migrations.StatusDone, since).Count(&done)
	ctrl.DB.WithContext(ctx).Model(&migrations.Task{}).
		Where("user_id = ? AND status = ? AND updated_at >= ?", user.ID, migrations.StatusLate, since).Count(&late)

	var minutes struct{ Total int64 }
	ctrl.DB.WithContext(ctx).Model(&migrations.StudySession{}).
		Select("COALESCE(SUM(duration),0) as total").
		Where("user_id = ? AND status = ? AND session_date >= ?", user.ID, migrations.SessionCompleted, since).
		Scan(&minutes)

	response.OK(konteks, "statistik rentang", gin.H{
		"since":         since.Format(time.RFC3339),
		"tasks_created": created,
		"tasks_done":    done,
		"tasks_late":    late,
		"study_minutes": minutes.Total,
		"study_hours":   float64(minutes.Total) / 60,
	})
}

func (ctrl *StatisticsController) GetBySubjects(konteks *gin.Context) {
	user := currentUser(konteks)

	type subjectStat struct {
		SubjectID uint    `json:"subject_id"`
		Name      string  `json:"name"`
		Total     int64   `json:"total_tasks"`
		Done      int64   `json:"done_tasks"`
		Late      int64   `json:"late_tasks"`
		AvgScore  float64 `json:"avg_priority_score"`
	}

	var stats []subjectStat
	err := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.Task{}).
		Select(`tasks.subject_id,
			COALESCE(subjects.name, 'Tanpa Mapel') as name,
			COUNT(*) as total,
			SUM(CASE WHEN tasks.status = ? THEN 1 ELSE 0 END) as done,
			SUM(CASE WHEN tasks.status = ? THEN 1 ELSE 0 END) as late,
			COALESCE(AVG(tasks.priority_score),0) as avg_score`,
			migrations.StatusDone, migrations.StatusLate).
		Joins("LEFT JOIN subjects ON subjects.id = tasks.subject_id").
		Where("tasks.user_id = ?", user.ID).
		Group("tasks.subject_id, subjects.name").
		Scan(&stats).Error
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "statistik per mata pelajaran", stats)
}
