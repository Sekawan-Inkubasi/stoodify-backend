package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type StudySessionController struct {
	DB *gorm.DB
}

func NewStudySessionController(db *gorm.DB) *StudySessionController {
	return &StudySessionController{DB: db}
}

type SessionCreateInput struct {
	TaskID         uint      `json:"task_id" binding:"required"`
	SessionDate    time.Time `json:"session_date" binding:"required"`
	StartTime      string    `json:"start_time" binding:"required,len=5"`
	Duration       int       `json:"duration" binding:"required,min=5,max=600"`
	TargetProgress int       `json:"target_progress" binding:"omitempty,min=0,max=100"`
}

type SessionUpdateInput struct {
	SessionDate *time.Time `json:"session_date"`
	StartTime   *string    `json:"start_time" binding:"omitempty,len=5"`
	Duration    *int       `json:"duration" binding:"omitempty,min=5,max=600"`
	Status      *string    `json:"status" binding:"omitempty,oneof=planned completed skipped"`
}

func (ctrl *StudySessionController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	var input SessionCreateInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if hhmmMinutes(input.StartTime) < 0 {
		response.UnprocessableEntity(konteks, "format start_time tidak valid", map[string]string{
			"start_time": "harus format HH:MM",
		})
		return
	}

	task, found := findOwnedTask(user.ID, input.TaskID)
	if !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	targetProgress := input.TargetProgress
	if targetProgress <= 0 {
		targetProgress = task.Progress + 20
		if targetProgress > 100 {
			targetProgress = 100
		}
	}

	session := migrations.StudySession{
		UserID:         user.ID,
		TaskID:         input.TaskID,
		SessionDate:    input.SessionDate,
		StartTime:      input.StartTime,
		Duration:       input.Duration,
		TargetProgress: targetProgress,
		Status:         migrations.SessionPlanned,
		AIGenerated:    false,
		Reason:         "Dibuat manual oleh siswa",
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&session).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	ctrl.DB.Preload("Task").First(&session, session.ID)
	response.Created(konteks, "sesi belajar dibuat", session)
}

func (ctrl *StudySessionController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)
	page, perPage, offset := params.Pagination(konteks, 50, 200)

	query := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.StudySession{}).Preload("Task").Where("user_id = ?", user.ID)

	if status := konteks.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var sessions []migrations.StudySession
	if err := query.Order("session_date ASC, start_time ASC").Limit(perPage).Offset(offset).Find(&sessions).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OKWithMeta(konteks, "daftar sesi belajar", sessions, response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
	})
}

// GetToday: sesi pada tanggal hari ini.
func (ctrl *StudySessionController) GetToday(konteks *gin.Context) {
	user := currentUser(konteks)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.AddDate(0, 0, 1)

	var sessions []migrations.StudySession
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Task").
		Where("user_id = ? AND session_date >= ? AND session_date < ?", user.ID, startOfDay, endOfDay).
		Order("start_time ASC").
		Find(&sessions).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "sesi belajar hari ini", sessions)
}

// Update: reschedule manual / ubah status sesi.
func (ctrl *StudySessionController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input SessionUpdateInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var session migrations.StudySession
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&session).Error; err != nil {
		response.NotFound(konteks, "sesi belajar tidak ditemukan")
		return
	}

	if input.SessionDate != nil {
		session.SessionDate = *input.SessionDate
	}
	if input.StartTime != nil {
		if hhmmMinutes(*input.StartTime) < 0 {
			response.UnprocessableEntity(konteks, "format start_time tidak valid", map[string]string{
				"start_time": "harus HH:MM",
			})
			return
		}
		session.StartTime = *input.StartTime
		session.AIGenerated = false // diubah manual oleh user
	}
	if input.Duration != nil {
		session.Duration = *input.Duration
	}
	if input.Status != nil {
		session.Status = *input.Status
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&session).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "sesi belajar diperbarui", session)
}

func (ctrl *StudySessionController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var session migrations.StudySession
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&session).Error; err != nil {
		response.NotFound(konteks, "sesi belajar tidak ditemukan")
		return
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&session).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.NoContent(konteks)
}
