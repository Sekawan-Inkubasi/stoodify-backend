package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type NotificationController struct {
	DB *gorm.DB
}

func NewNotificationController(db *gorm.DB) *NotificationController {
	return &NotificationController{DB: db}
}

func (ctrl *NotificationController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)
	page, perPage, offset := params.Pagination(konteks, 50, 200)

	query := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.Notification{}).Where("user_id = ?", user.ID)

	if konteks.Query("unread") == "true" {
		query = query.Where("is_read = ?", false)
	}

	var total int64
	query.Count(&total)

	var notifications []migrations.Notification
	if err := query.Order("scheduled_at DESC, id DESC").Limit(perPage).Offset(offset).Find(&notifications).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OKWithMeta(konteks, "daftar notifikasi", notifications, response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
	})
}

func (ctrl *NotificationController) MarkAsRead(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	result := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.Notification{}).
		Where("id = ? AND user_id = ?", id, user.ID).
		Update("is_read", true)
	if result.Error != nil {
		response.ServerError(konteks, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(konteks, "notifikasi tidak ditemukan")
		return
	}
	response.OK(konteks, "notifikasi ditandai dibaca", nil)
}

func (ctrl *NotificationController) MarkAllAsRead(konteks *gin.Context) {
	user := currentUser(konteks)

	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.Notification{}).
		Where("user_id = ? AND is_read = ?", user.ID, false).
		Update("is_read", true).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "semua notifikasi ditandai dibaca", nil)
}

// createNotification menulis satu baris notifikasi.
func createNotification(db *gorm.DB, userID uint, taskID *uint, title, message, notifType string, scheduledAt time.Time) error {
	return db.Create(&migrations.Notification{
		UserID:      userID,
		TaskID:      taskID,
		Title:       title,
		Message:     message,
		Type:        notifType,
		ScheduledAt: scheduledAt,
	}).Error
}
