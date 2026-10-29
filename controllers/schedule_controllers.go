package controllers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type ScheduleController struct {
	DB *gorm.DB
}

func NewScheduleController(db *gorm.DB) *ScheduleController {
	return &ScheduleController{DB: db}
}

type ScheduleInput struct {
	SubjectID uint   `json:"subject_id" binding:"required"`
	DayOfWeek string `json:"day_of_week" binding:"required"`
	StartTime string `json:"start_time" binding:"required,len=5"`
	EndTime   string `json:"end_time" binding:"required,len=5"`
	Room      string `json:"room" binding:"omitempty,max=50"`
	Teacher   string `json:"teacher" binding:"omitempty,max=100"`
}

func normalizeDayOfWeek(day string) string {
	switch strings.ToLower(strings.TrimSpace(day)) {
	case "senin", "monday", "1":
		return "Monday"
	case "selasa", "tuesday", "2":
		return "Tuesday"
	case "rabu", "wednesday", "3":
		return "Wednesday"
	case "kamis", "thursday", "4":
		return "Thursday"
	case "jumat", "jum'at", "friday", "5":
		return "Friday"
	case "sabtu", "saturday", "6":
		return "Saturday"
	case "minggu", "ahad", "sunday", "7":
		return "Sunday"
	default:
		return ""
	}
}

func (ctrl *ScheduleController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)

	var schedules []migrations.Schedule
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", user.ID).
		Order("day_of_week, start_time").
		Find(&schedules).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "daftar jadwal", schedules)
}

func (ctrl *ScheduleController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	var input ScheduleInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	day := normalizeDayOfWeek(input.DayOfWeek)
	if day == "" {
		response.UnprocessableEntity(konteks, "nama hari tidak valid", map[string]string{
			"day_of_week": "harus nama hari (Senin-Minggu / Monday-Sunday / 1-7)",
		})
		return
	}

	start, end := hhmmMinutes(input.StartTime), hhmmMinutes(input.EndTime)
	if start < 0 || end < 0 || start >= end {
		response.UnprocessableEntity(konteks, "rentang waktu jadwal tidak valid", map[string]string{
			"end_time": "harus format HH:MM dan lebih besar dari start_time",
		})
		return
	}

	if _, found := findOwnedSubject(user.ID, input.SubjectID); !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	conflict, err := ctrl.findConflict(konteks, user.ID, day, start, end, 0)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	if conflict != nil {
		response.Fail(konteks, 409, "jadwal bentrok dengan jadwal lain di hari yang sama", map[string]string{
			"conflict_with": conflict.StartTime + "-" + conflict.EndTime,
		})
		return
	}

	schedule := migrations.Schedule{
		UserID:    user.ID,
		SubjectID: input.SubjectID,
		DayOfWeek: day,
		StartTime: input.StartTime,
		EndTime:   input.EndTime,
		Room:      input.Room,
		Teacher:   input.Teacher,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&schedule).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	ctrl.DB.Preload("Subject").First(&schedule, schedule.ID)
	response.Created(konteks, "jadwal dibuat", schedule)
}

func (ctrl *ScheduleController) GetToday(konteks *gin.Context) {
	ctrl.byDay(konteks, time.Now().Weekday().String())
}

func (ctrl *ScheduleController) GetWeek(konteks *gin.Context) {
	user := currentUser(konteks)

	var schedules []migrations.Schedule
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subject").
		Where("user_id = ?", user.ID).
		Order("case day_of_week " +
			"when 'Monday' then 1 when 'Tuesday' then 2 when 'Wednesday' then 3 " +
			"when 'Thursday' then 4 when 'Friday' then 5 when 'Saturday' then 6 else 7 end, start_time").
		Find(&schedules).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "jadwal sepekan", schedules)
}

func (ctrl *ScheduleController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input ScheduleInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var schedule migrations.Schedule
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&schedule).Error; err != nil {
		response.NotFound(konteks, "jadwal tidak ditemukan")
		return
	}

	day := normalizeDayOfWeek(input.DayOfWeek)
	if day == "" {
		response.UnprocessableEntity(konteks, "nama hari tidak valid", map[string]string{
			"day_of_week": "harus nama hari (Senin-Minggu / Monday-Sunday / 1-7)",
		})
		return
	}

	start, end := hhmmMinutes(input.StartTime), hhmmMinutes(input.EndTime)
	if start < 0 || end < 0 || start >= end {
		response.UnprocessableEntity(konteks, "rentang waktu jadwal tidak valid", map[string]string{
			"end_time": "harus format HH:MM dan lebih besar dari start_time",
		})
		return
	}
	if _, found := findOwnedSubject(user.ID, input.SubjectID); !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	conflict, err := ctrl.findConflict(konteks, user.ID, day, start, end, schedule.ID)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	if conflict != nil {
		response.Fail(konteks, 409, "jadwal bentrok dengan jadwal lain di hari yang sama", map[string]string{
			"conflict_with": conflict.StartTime + "-" + conflict.EndTime,
		})
		return
	}

	schedule.SubjectID = input.SubjectID
	schedule.DayOfWeek = day
	schedule.StartTime = input.StartTime
	schedule.EndTime = input.EndTime
	schedule.Room = input.Room
	schedule.Teacher = input.Teacher

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&schedule).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	ctrl.DB.Preload("Subject").First(&schedule, schedule.ID)
	response.OK(konteks, "jadwal diperbarui", schedule)
}

func (ctrl *ScheduleController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var schedule migrations.Schedule
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("id = ? AND user_id = ?", id, user.ID).First(&schedule).Error; err != nil {
		response.NotFound(konteks, "jadwal tidak ditemukan")
		return
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&schedule).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.NoContent(konteks)
}

func (ctrl *ScheduleController) byDay(konteks *gin.Context, day string) {
	user := currentUser(konteks)

	var schedules []migrations.Schedule
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subject").
		Where("user_id = ? AND day_of_week = ?", user.ID, strings.TrimSpace(day)).
		Order("start_time").
		Find(&schedules).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "jadwal hari "+day, schedules)
}

// findConflict mencari jadwal lain di hari yang sama dengan rentang waktu beririsan.
func (ctrl *ScheduleController) findConflict(konteks *gin.Context, userID uint, day string, start, end int, excludeID uint) (*migrations.Schedule, error) {
	var schedules []migrations.Schedule
	query := ctrl.DB.WithContext(konteks.Request.Context()).Where("user_id = ? AND day_of_week = ?", userID, day)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Find(&schedules).Error; err != nil {
		return nil, err
	}
	for i := range schedules {
		otherStart, otherEnd := hhmmMinutes(schedules[i].StartTime), hhmmMinutes(schedules[i].EndTime)
		if otherStart < 0 || otherEnd < 0 {
			continue
		}
		if overlaps(start, end, otherStart, otherEnd) {
			return &schedules[i], nil
		}
	}
	return nil, nil
}
