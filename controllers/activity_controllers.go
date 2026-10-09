package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type ActivityController struct {
	DB *gorm.DB
}

func NewActivityController(db *gorm.DB) *ActivityController {
	return &ActivityController{DB: db}
}

type ActivityInput struct {
	Title      string `json:"title" binding:"required,min=1,max=200"`
	StartTime  string `json:"start_time" binding:"required,len=5"`
	EndTime    string `json:"end_time" binding:"required,len=5"`
	Recurrence string `json:"recurrence" binding:"omitempty,oneof=daily weekly monthly none"`
}

func (ctrl *ActivityController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)

	var activities []migrations.Activity
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ?", user.ID).Order("start_time ASC").Find(&activities).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "daftar aktivitas", activities)
}

func (ctrl *ActivityController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	var input ActivityInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	start, end := hhmmMinutes(input.StartTime), hhmmMinutes(input.EndTime)
	if start < 0 || end < 0 || start >= end {
		response.UnprocessableEntity(konteks, "rentang waktu aktivitas tidak valid", map[string]string{
			"end_time": "harus format HH:MM dan lebih besar dari start_time",
		})
		return
	}

	recurrence := input.Recurrence
	if recurrence == "" {
		recurrence = "weekly"
	}

	activity := migrations.Activity{
		UserID:     user.ID,
		Title:      input.Title,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		Recurrence: recurrence,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&activity).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.Created(konteks, "aktivitas dibuat", activity)
}

func (ctrl *ActivityController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input ActivityInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var activity migrations.Activity
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&activity).Error; err != nil {
		response.NotFound(konteks, "aktivitas tidak ditemukan")
		return
	}

	start, end := hhmmMinutes(input.StartTime), hhmmMinutes(input.EndTime)
	if start < 0 || end < 0 || start >= end {
		response.UnprocessableEntity(konteks, "rentang waktu aktivitas tidak valid", map[string]string{
			"end_time": "harus format HH:MM dan lebih besar dari start_time",
		})
		return
	}

	activity.Title = input.Title
	activity.StartTime = input.StartTime
	activity.EndTime = input.EndTime
	if input.Recurrence != "" {
		activity.Recurrence = input.Recurrence
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&activity).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "aktivitas diperbarui", activity)
}

func (ctrl *ActivityController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var activity migrations.Activity
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&activity).Error; err != nil {
		response.NotFound(konteks, "aktivitas tidak ditemukan")
		return
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&activity).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.NoContent(konteks)
}
