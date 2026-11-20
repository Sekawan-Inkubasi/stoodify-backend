package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/priority"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type TaskController struct {
	DB *gorm.DB
}

func NewTaskController(db *gorm.DB) *TaskController {
	return &TaskController{DB: db}
}

type TaskInput struct {
	SubjectID         uint      `json:"subject_id" binding:"required"`
	Title             string    `json:"title" binding:"required,min=1,max=200"`
	MaterialName      string    `json:"material_name" binding:"omitempty,max=100"`
	Description       string    `json:"description" binding:"omitempty,max=2000"`
	Type              string    `json:"type" binding:"omitempty,oneof=exam project assignment reading"`
	Deadline          time.Time `json:"deadline" binding:"required"`
	Difficulty        int       `json:"difficulty" binding:"required,min=1,max=5"`
	EstimatedDuration int       `json:"estimated_duration" binding:"omitempty,min=5,max=1440"`
	Notes             string    `json:"notes" binding:"omitempty,max=2000"`
}

// TaskUpdateInput: semua field opsional — hanya field yang dikirim yang diubah (PATCH).
type TaskUpdateInput struct {
	SubjectID         *uint      `json:"subject_id" binding:"omitempty"`
	Title             *string    `json:"title" binding:"omitempty,min=1,max=200"`
	MaterialName      *string    `json:"material_name" binding:"omitempty,max=100"`
	Description       *string    `json:"description" binding:"omitempty,max=2000"`
	Type              *string    `json:"type" binding:"omitempty,oneof=exam project assignment reading"`
	Deadline          *time.Time `json:"deadline" binding:"omitempty"`
	Difficulty        *int       `json:"difficulty" binding:"omitempty,min=1,max=5"`
	EstimatedDuration *int       `json:"estimated_duration" binding:"omitempty,min=5,max=1440"`
	Notes             *string    `json:"notes" binding:"omitempty,max=2000"`
}

type TaskResponse struct {
	migrations.Task
	PriorityBreakdown *priority.Breakdown `json:"priority_breakdown,omitempty"`
}

func toTaskResponse(task migrations.Task, budget int) TaskResponse {
	bd := priority.CalculateBreakdown(priority.Input{
		Deadline:        task.Deadline,
		Difficulty:      task.Difficulty,
		EstDuration:     task.EstimatedDuration,
		Type:            task.Type,
		Progress:        task.Progress,
		MaxDailyMinutes: budget,
		Now:             time.Now(),
	})
	return TaskResponse{
		Task:              task,
		PriorityBreakdown: &bd,
	}
}

type TaskStatusInput struct {
	Status string `json:"status" binding:"required,oneof=pending in_progress done late"`
}

type TaskProgressInput struct {
	Progress *int `json:"progress" binding:"required,min=0,max=100"`
}

// GetAll mendukung filter ?status= &subject_id=, search judul, sort prioritas.
func (ctrl *TaskController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)
	page, perPage, offset := params.Pagination(konteks, 20, 100)

	query := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.Task{}).Preload("Subtasks").Where("user_id = ?", user.ID)

	if status := konteks.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if subjectID := konteks.Query("subject_id"); subjectID != "" {
		query = query.Where("subject_id = ?", subjectID)
	}
	if search := konteks.Query("search"); search != "" {
		query = query.Where("title ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	order := "deadline ASC"
	if konteks.Query("sort") == "priority" {
		order = "priority_score DESC, deadline ASC"
	}

	var tasks []migrations.Task
	if err := query.Order(order).Limit(perPage).Offset(offset).Find(&tasks).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	budget := dailyBudgetMinutes(user.ID)

	taskResponses := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		taskResponses[i] = toTaskResponse(t, budget)
	}

	response.OKWithMeta(konteks, "daftar tugas", taskResponses, response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
	})
}

func (ctrl *TaskController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	var input TaskInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}
	if _, found := findOwnedSubject(user.ID, input.SubjectID); !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	taskType := input.Type
	if taskType == "" {
		taskType = "assignment"
	}
	duration := input.EstimatedDuration
	if duration == 0 {
		duration = 60
	}

	task := migrations.Task{
		UserID:            user.ID,
		SubjectID:         input.SubjectID,
		Title:             input.Title,
		MaterialName:      input.MaterialName,
		Description:       input.Description,
		Type:              taskType,
		Deadline:          input.Deadline,
		Difficulty:        input.Difficulty,
		EstimatedDuration: duration,
		Status:            migrations.StatusPending,
		Notes:             input.Notes,
	}
	computeTaskPriority(&task)

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&task).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.Created(konteks, "tugas dibuat", task)
}

func (ctrl *TaskController) GetByID(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var task migrations.Task
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subtasks").Preload("Subject").
		Where("id = ? AND user_id = ?", id, user.ID).First(&task).Error; err != nil {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}
	response.OK(konteks, "detail tugas", task)
}

func (ctrl *TaskController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input TaskUpdateInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	task, found := findOwnedTask(user.ID, id)
	if !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	// Terapkan hanya field yang dikirim (PATCH parsial).
	if input.SubjectID != nil {
		if _, ok := findOwnedSubject(user.ID, *input.SubjectID); !ok {
			response.NotFound(konteks, "mata pelajaran tidak ditemukan")
			return
		}
		task.SubjectID = *input.SubjectID
	}
	if input.Title != nil {
		task.Title = *input.Title
	}
	if input.MaterialName != nil {
		task.MaterialName = *input.MaterialName
	}
	if input.Description != nil {
		task.Description = *input.Description
	}
	if input.Type != nil {
		task.Type = *input.Type
	}
	if input.Deadline != nil {
		task.Deadline = *input.Deadline
	}
	if input.Difficulty != nil {
		task.Difficulty = *input.Difficulty
	}
	if input.EstimatedDuration != nil {
		task.EstimatedDuration = *input.EstimatedDuration
	}
	if input.Notes != nil {
		task.Notes = *input.Notes
	}
	recalcTask(&task)

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&task).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.OK(konteks, "tugas diperbarui", task)
}

func (ctrl *TaskController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	task, found := findOwnedTask(user.ID, id)
	if !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&task).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.NoContent(konteks)
}

func (ctrl *TaskController) UpdateStatus(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input TaskStatusInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	task, found := findOwnedTask(user.ID, id)
	if !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	task.Status = input.Status
	if input.Status == migrations.StatusDone {
		task.Progress = 100
	}
	computeTaskPriority(&task)
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&task).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.OK(konteks, "status tugas diperbarui", task)
}

func (ctrl *TaskController) UpdateProgress(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input TaskProgressInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	task, found := findOwnedTask(user.ID, id)
	if !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	progressVal := *input.Progress
	task.Progress = progressVal
	switch {
	case progressVal >= 100:
		task.Status = migrations.StatusDone
	case task.Status == migrations.StatusDone:
		task.Status = migrations.StatusInProgress
	case task.Status == migrations.StatusPending && progressVal > 0:
		task.Status = migrations.StatusInProgress
	}
	computeTaskPriority(&task)

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&task).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.OK(konteks, "progres tugas diperbarui", task)
}

// GetUpcoming: deadline dalam 7 hari ke depan, status belum selesai (terdekat dulu).
func (ctrl *TaskController) GetUpcoming(konteks *gin.Context) {
	ctrl.byDeadlineWindow(konteks, time.Now(), time.Now().AddDate(0, 0, 7), "ASC")
}

// GetOverdue: deadline sudah lewat dan belum selesai.
func (ctrl *TaskController) GetOverdue(konteks *gin.Context) {
	ctrl.byDeadlineWindow(konteks, time.Time{}, time.Now(), "ASC")
}

func (ctrl *TaskController) byDeadlineWindow(konteks *gin.Context, from, to time.Time, order string) {
	user := currentUser(konteks)

	query := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subject").
		Where("user_id = ? AND status <> ?", user.ID, migrations.StatusDone)
	if !from.IsZero() {
		query = query.Where("deadline >= ?", from)
	}
	query = query.Where("deadline <= ?", to)

	var tasks []migrations.Task
	if err := query.Order("deadline " + order).Find(&tasks).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "daftar tugas", tasks)
}

// markLateTasks mengubah status tugas yang lewat deadline menjadi "late".
func markLateTasks(db *gorm.DB) (int64, error) {
	result := db.Model(&migrations.Task{}).
		Where("deadline < ? AND status IN ?", time.Now(), []string{migrations.StatusPending, migrations.StatusInProgress}).
		Updates(map[string]interface{}{"status": migrations.StatusLate})
	return result.RowsAffected, result.Error
}
