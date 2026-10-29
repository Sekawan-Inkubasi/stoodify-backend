package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type SubjectController struct {
	DB *gorm.DB
}

func NewSubjectController(db *gorm.DB) *SubjectController {
	return &SubjectController{DB: db}
}

type SubjectInput struct {
	Name       string `json:"name" binding:"required,min=1,max=100"`
	Code       string `json:"code" binding:"omitempty,max=20"`
	Teacher    string `json:"teacher" binding:"omitempty,max=100"`
	Color      string `json:"color" binding:"omitempty,max=20"`
	AccentBg   string `json:"accent_bg" binding:"omitempty,max=30"`
	Category   string `json:"category" binding:"omitempty,max=50"`
	Difficulty int    `json:"difficulty" binding:"omitempty,min=1,max=5"`
}

// GetAll mendukung ?search= dan pagination.
func (ctrl *SubjectController) GetAll(konteks *gin.Context) {
	user := currentUser(konteks)
	page, perPage, offset := params.Pagination(konteks, 50, 200)

	query := ctrl.DB.WithContext(konteks.Request.Context()).Model(&migrations.Subject{}).Where("user_id = ?", user.ID)
	if search := konteks.Query("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var subjects []migrations.Subject
	if err := query.Order("name ASC").Limit(perPage).Offset(offset).Find(&subjects).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OKWithMeta(konteks, "daftar mata pelajaran", subjects, response.Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages(total, perPage),
	})
}

func (ctrl *SubjectController) Create(konteks *gin.Context) {
	user := currentUser(konteks)
	var input SubjectInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	subject := migrations.Subject{
		UserID:     user.ID,
		Name:       input.Name,
		Code:       input.Code,
		Teacher:    input.Teacher,
		Color:      input.Color,
		AccentBg:   input.AccentBg,
		Category:   input.Category,
		Difficulty: input.Difficulty,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&subject).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.Created(konteks, "mata pelajaran dibuat", subject)
}

func (ctrl *SubjectController) GetByID(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	subject, found := findOwnedSubject(user.ID, id)
	if !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}
	response.OK(konteks, "detail mata pelajaran", subject)
}

func (ctrl *SubjectController) Update(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input SubjectInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	subject, found := findOwnedSubject(user.ID, id)
	if !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	subject.Name = input.Name
	subject.Code = input.Code
	subject.Teacher = input.Teacher
	subject.Color = input.Color
	subject.AccentBg = input.AccentBg
	subject.Category = input.Category
	if input.Difficulty > 0 {
		subject.Difficulty = input.Difficulty
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&subject).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "mata pelajaran diperbarui", subject)
}

func (ctrl *SubjectController) Delete(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	subject, found := findOwnedSubject(user.ID, id)
	if !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Delete(&subject).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.NoContent(konteks)
}

func totalPages(total int64, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := int(total) / perPage
	if int(total)%perPage != 0 {
		pages++
	}
	return pages
}
