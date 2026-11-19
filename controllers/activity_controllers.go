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
	// DayOfWeek opsional (Monday..Sunday / Senin..Minggu / 1..7); wajib untuk weekly.
	DayOfWeek string `json:"day_of_week" binding:"omitempty"`
}

// ActivityUpdateInput: semua field opsional — hanya field yang dikirim yang diubah (PATCH).
type ActivityUpdateInput struct {
	Title      *string `json:"title" binding:"omitempty,min=1,max=200"`
	StartTime  *string `json:"start_time" binding:"omitempty,len=5"`
	EndTime    *string `json:"end_time" binding:"omitempty,len=5"`
	Recurrence *string `json:"recurrence" binding:"omitempty,oneof=daily weekly monthly none"`
	DayOfWeek  *string `json:"day_of_week" binding:"omitempty"`
}

// hariAktivitas menormalkan & memvalidasi day_of_week aktivitas.
// Mengembalikan ok=false bila nilainya tidak dikenali.
func hariAktivitas(nilai string) (string, bool) {
	if nilai == "" {
		return "", true
	}
	hari := normalizeDayOfWeek(nilai)
	return hari, hari != ""
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

	hari, valid := hariAktivitas(input.DayOfWeek)
	if !valid {
		response.UnprocessableEntity(konteks, "nama hari tidak valid", map[string]string{
			"day_of_week": "harus nama hari (Senin-Minggu / Monday-Sunday / 1-7)",
		})
		return
	}

	activity := migrations.Activity{
		UserID:     user.ID,
		Title:      input.Title,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		Recurrence: recurrence,
		DayOfWeek:  hari,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&activity).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.Created(konteks, "aktivitas dibuat", activity)
}

func (ctrl *ActivityController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input ActivityUpdateInput
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

	// Terapkan hanya field yang dikirim (PATCH parsial).
	if input.Title != nil {
		activity.Title = *input.Title
	}
	if input.StartTime != nil {
		activity.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		activity.EndTime = *input.EndTime
	}
	if input.Recurrence != nil {
		activity.Recurrence = *input.Recurrence
	}
	if input.DayOfWeek != nil {
		hari, valid := hariAktivitas(*input.DayOfWeek)
		if !valid {
			response.UnprocessableEntity(konteks, "nama hari tidak valid", map[string]string{
				"day_of_week": "harus nama hari (Senin-Minggu / Monday-Sunday / 1-7)",
			})
			return
		}
		activity.DayOfWeek = hari
	}

	start, end := hhmmMinutes(activity.StartTime), hhmmMinutes(activity.EndTime)
	if start < 0 || end < 0 || start >= end {
		response.UnprocessableEntity(konteks, "rentang waktu aktivitas tidak valid", map[string]string{
			"end_time": "harus format HH:MM dan lebih besar dari start_time",
		})
		return
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&activity).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
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
	picuRegenerasi(user.ID)
	response.NoContent(konteks)
}
