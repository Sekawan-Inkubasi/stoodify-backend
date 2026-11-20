package controllers

import (
	"time"

	"github.com/gin-gonic/gin"

	"stoodify_backend/config"
	"stoodify_backend/internal/priority"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

// Helper lintas controller.

func currentUser(konteks *gin.Context) migrations.User {
	return middlewares.MustUser(konteks)
}

// dailyBudgetMinutes: total anggaran belajar harian dari preferensi user (fallback 120).
// Dipakai konsisten oleh priority_score (tersimpan) dan priority_breakdown (respons GET).
func dailyBudgetMinutes(userID uint) int {
	var pref migrations.UserPreference
	err := config.DB.Where("user_id = ?", userID).First(&pref).Error
	if err != nil || pref.SessionDuration <= 0 {
		return 120
	}
	maxSessions := pref.MaxSessionsPerDay
	if maxSessions <= 0 {
		maxSessions = 3
	}
	return pref.SessionDuration * maxSessions
}

// computeTaskPriority menghitung priority_score dan mengisi task.PriorityScore tanpa
// menulis ke DB (aman dipanggil sebelum Create atau Save).
func computeTaskPriority(task *migrations.Task) float64 {
	score := priority.Score(priority.Input{
		Deadline:        task.Deadline,
		Difficulty:      task.Difficulty,
		EstDuration:     task.EstimatedDuration,
		Type:            task.Type,
		Progress:        task.Progress,
		MaxDailyMinutes: dailyBudgetMinutes(task.UserID),
		Now:             time.Now(),
	})
	task.PriorityScore = score
	return score
}

// recalcTask menghitung ulang priority_score dan menyimpannya bila task sudah punya ID.
func recalcTask(task *migrations.Task) {
	score := computeTaskPriority(task)
	if task.ID > 0 {
		config.DB.Model(task).UpdateColumn("priority_score", score)
	}
}

func findOwnedTask(userID, taskID uint) (migrations.Task, bool) {
	var task migrations.Task
	err := config.DB.Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error
	return task, err == nil
}

func findOwnedSubject(userID, subjectID uint) (migrations.Subject, bool) {
	var subject migrations.Subject
	err := config.DB.Where("id = ? AND user_id = ?", subjectID, userID).First(&subject).Error
	return subject, err == nil
}

// hhmmMinutes mengubah "19:00" menjadi menit sejak tengah malam; -1 kalau tidak valid.
func hhmmMinutes(value string) int {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

// overlaps mendeteksi bentrok dua rentang waktu pada hari yang sama.
func overlaps(startA, endA, startB, endB int) bool {
	return startA < endB && startB < endA
}
