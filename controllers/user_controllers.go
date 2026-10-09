package controllers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type UserController struct {
	DB *gorm.DB
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{DB: db}
}

type UpdateMeInput struct {
	Name  *string `json:"name" binding:"omitempty,min=1,max=100"`
	Email *string `json:"email" binding:"omitempty,email,max=100"`
}

type PreferencesInput struct {
	StartTime       string `json:"start_time" binding:"omitempty,len=5"`
	EndTime         string `json:"end_time" binding:"omitempty,len=5"`
	SessionDuration int    `json:"session_duration" binding:"omitempty,min=15,max=600"`
	ActiveDays      string `json:"active_days"`
}

func (ctrl *UserController) GetMe(konteks *gin.Context) {
	response.OK(konteks, "data user", currentUser(konteks))
}

func (ctrl *UserController) UpdateMe(konteks *gin.Context) {
	user := currentUser(konteks)
	var input UpdateMeInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if input.Name != nil {
		user.Name = *input.Name
	}
	if input.Email != nil {
		email := normalizeEmail(*input.Email)
		var other migrations.User
		if err := ctrl.DB.Where("email = ? AND id <> ?", email, user.ID).First(&other).Error; err == nil {
			response.Fail(konteks, 409, "email sudah dipakai user lain", nil)
			return
		}
		user.Email = email
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&user).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "profil diperbarui", user)
}

func (ctrl *UserController) GetPreferences(konteks *gin.Context) {
	user := currentUser(konteks)

	var pref migrations.UserPreference
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("user_id = ?", user.ID).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.OK(konteks, "preferensi belum diatur", migrations.UserPreference{UserID: user.ID})
		return
	}
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "preferensi user", pref)
}

func (ctrl *UserController) UpdatePreferences(konteks *gin.Context) {
	user := currentUser(konteks)
	var input PreferencesInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var pref migrations.UserPreference
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("user_id = ?", user.ID).First(&pref).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ServerError(konteks, err)
		return
	}

	pref.UserID = user.ID
	if input.StartTime != "" {
		pref.StartTime = input.StartTime
	}
	if input.EndTime != "" {
		pref.EndTime = input.EndTime
	}
	if input.SessionDuration > 0 {
		pref.SessionDuration = input.SessionDuration
	}
	if input.ActiveDays != "" {
		pref.ActiveDays = input.ActiveDays
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&pref).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "preferensi diperbarui", pref)
}
