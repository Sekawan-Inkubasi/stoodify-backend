package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type SubtaskController struct {
	DB *gorm.DB
}

func NewSubtaskController(db *gorm.DB) *SubtaskController {
	return &SubtaskController{DB: db}
}

type SubtaskInput struct {
	Title string `json:"title" binding:"required,min=1,max=200"`
}

type SubtaskUpdateInput struct {
	Title       *string `json:"title" binding:"omitempty,min=1,max=200"`
	IsCompleted *bool   `json:"is_completed"`
}

func (ctrl *SubtaskController) GetAllByTask(konteks *gin.Context) {
	user := currentUser(konteks)
	taskID, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}
	if _, found := findOwnedTask(user.ID, taskID); !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	var subtasks []migrations.Subtask
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("task_id = ?", taskID).Order("id ASC").Find(&subtasks).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "daftar subtugas", subtasks)
}

func (ctrl *SubtaskController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	taskID, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}
	if _, found := findOwnedTask(user.ID, taskID); !found {
		response.NotFound(konteks, "tugas tidak ditemukan")
		return
	}

	var input SubtaskInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	subtask := migrations.Subtask{TaskID: taskID, Title: input.Title}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&subtask).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.Created(konteks, "subtugas dibuat", subtask)
}

func (ctrl *SubtaskController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input SubtaskUpdateInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var subtask migrations.Subtask
	if err := ctrl.DB.WithContext(konteks.Request.Context()).First(&subtask, id).Error; err != nil {
		response.NotFound(konteks, "subtugas tidak ditemukan")
		return
	}
	// Kepemilikan dicek lewat task induk.
	if _, found := findOwnedTask(user.ID, subtask.TaskID); !found {
		response.NotFound(konteks, "subtugas tidak ditemukan")
		return
	}

	if input.Title != nil {
		subtask.Title = *input.Title
	}
	if input.IsCompleted != nil {
		subtask.IsCompleted = *input.IsCompleted
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&subtask).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "subtugas diperbarui", subtask)
}

func (ctrl *SubtaskController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var subtask migrations.Subtask
	if err := ctrl.DB.WithContext(konteks.Request.Context()).First(&subtask, id).Error; err != nil {
		response.NotFound(konteks, "subtugas tidak ditemukan")
		return
	}
	if _, found := findOwnedTask(user.ID, subtask.TaskID); !found {
		response.NotFound(konteks, "subtugas tidak ditemukan")
		return
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&subtask).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.NoContent(konteks)
}
