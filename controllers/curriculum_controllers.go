package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type CurriculumController struct {
	DB *gorm.DB
}

func NewCurriculumController(db *gorm.DB) *CurriculumController {
	return &CurriculumController{DB: db}
}

type CurriculumTopicItemResponse struct {
	ID           uint       `json:"id"`
	SubjectID    uint       `json:"subject_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Order        int        `json:"order"`
	Grade        int8       `json:"grade"`
	Major        string     `json:"major"`
	Term         int8       `json:"term"`
	EstimatedMin int        `json:"estimated_minutes"`
	Status       string     `json:"status"`
	Notes        string     `json:"notes,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type RoadmapSummary struct {
	TotalTopics     int     `json:"total_topics"`
	MasteredTopics  int     `json:"mastered_topics"`
	InProgressCount int     `json:"in_progress_count"`
	CompletionRate  float64 `json:"completion_rate"`
}

type CurriculumRoadmapResponse struct {
	Subject migrations.Subject            `json:"subject"`
	Summary RoadmapSummary                 `json:"summary"`
	Topics  []CurriculumTopicItemResponse  `json:"topics"`
}

type TopicProgressInput struct {
	Status string `json:"status" binding:"required,oneof=not_started in_progress mastered"`
	Notes  string `json:"notes" binding:"omitempty,max=1000"`
}

type CreateTopicInput struct {
	SubjectID    uint   `json:"subject_id" binding:"required"`
	Title        string `json:"title" binding:"required,min=1,max=200"`
	Description  string `json:"description" binding:"omitempty,max=2000"`
	Order        int    `json:"order"`
	Grade        int8   `json:"grade" binding:"omitempty,min=7,max=13"`
	Major        string `json:"major" binding:"omitempty,max=100"`
	Term         int8   `json:"term" binding:"omitempty,min=1,max=4"`
	EstimatedMin int    `json:"estimated_minutes" binding:"omitempty,min=5,max=1440"`
}

// GetRoadmapBySubject mengambil daftar topik silabus dan progress belajar per subject (RoadmapView).
func (ctrl *CurriculumController) GetRoadmapBySubject(konteks *gin.Context) {
	user := currentUser(konteks)
	subjectID, ok := params.IDParam(konteks, "subjectId")
	if !ok {
		return
	}

	subject, found := findOwnedSubject(user.ID, subjectID)
	if !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	var topics []migrations.CurriculumTopic
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("subject_id = ?", subjectID).
		Order("`order` ASC, id ASC").
		Find(&topics).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	var topicIDs []uint
	for _, t := range topics {
		topicIDs = append(topicIDs, t.ID)
	}

	progressMap := make(map[uint]migrations.UserTopicProgress)
	if len(topicIDs) > 0 {
		var userProgress []migrations.UserTopicProgress
		ctrl.DB.WithContext(konteks.Request.Context()).
			Where("user_id = ? AND topic_id IN ?", user.ID, topicIDs).
			Find(&userProgress)
		for _, p := range userProgress {
			progressMap[p.TopicID] = p
		}
	}

	items := make([]CurriculumTopicItemResponse, len(topics))
	masteredCount := 0
	inProgressCount := 0

	for i, t := range topics {
		status := migrations.TopicStatusNotStarted
		notes := ""
		var completedAt *time.Time

		if p, exists := progressMap[t.ID]; exists {
			status = p.Status
			notes = p.Notes
			completedAt = p.CompletedAt
		}

		if status == migrations.TopicStatusMastered {
			masteredCount++
		} else if status == migrations.TopicStatusInProgress {
		inProgressCount++
		}

		items[i] = CurriculumTopicItemResponse{
			ID:           t.ID,
			SubjectID:    t.SubjectID,
			Title:        t.Title,
			Description:  t.Description,
			Order:        t.Order,
			Grade:        t.Grade,
			Major:        t.Major,
			Term:         t.Term,
			EstimatedMin: t.EstimatedMin,
			Status:       status,
			Notes:        notes,
			CompletedAt:  completedAt,
		}
	}

	total := len(topics)
	completionRate := 0.0
	if total > 0 {
		completionRate = float64(masteredCount) / float64(total) * 100.0
	}

	response.OK(konteks, "silabus dan roadmap mata pelajaran", CurriculumRoadmapResponse{
		Subject: subject,
		Summary: RoadmapSummary{
			TotalTopics:     total,
			MasteredTopics:  masteredCount,
			InProgressCount: inProgressCount,
			CompletionRate:  completionRate,
		},
		Topics: items,
	})
}

// CreateTopic menambahkan topik baru ke dalam kurikulum/silabus mata pelajaran.
func (ctrl *CurriculumController) CreateTopic(konteks *gin.Context) {
	user := currentUser(konteks)
	var input CreateTopicInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if _, found := findOwnedSubject(user.ID, input.SubjectID); !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	estMin := input.EstimatedMin
	if estMin <= 0 {
		estMin = 60
	}
	term := input.Term
	if term <= 0 {
		term = 1
	}

	topic := migrations.CurriculumTopic{
		SubjectID:    input.SubjectID,
		Title:        input.Title,
		Description:  input.Description,
		Order:        input.Order,
		Grade:        input.Grade,
		Major:        input.Major,
		Term:         term,
		EstimatedMin: estMin,
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&topic).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.Created(konteks, "topik kurikulum berhasil dibuat", topic)
}

// UpdateProgress memperbarui status progress belajar siswa pada topik tertentu.
func (ctrl *CurriculumController) UpdateProgress(konteks *gin.Context) {
	user := currentUser(konteks)
	topicID, ok := params.IDParam(konteks, "topicId")
	if !ok {
		return
	}

	var topic migrations.CurriculumTopic
	if err := ctrl.DB.WithContext(konteks.Request.Context()).First(&topic, topicID).Error; err != nil {
		response.NotFound(konteks, "topik kurikulum tidak ditemukan")
		return
	}

	if _, found := findOwnedSubject(user.ID, topic.SubjectID); !found {
		response.Forbidden(konteks, "tidak memiliki akses ke mata pelajaran topik ini")
		return
	}

	var input TopicProgressInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var progress migrations.UserTopicProgress
	err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ? AND topic_id = ?", user.ID, topic.ID).
		First(&progress).Error

	var completedAt *time.Time
	if input.Status == migrations.TopicStatusMastered {
		now := time.Now()
		completedAt = &now
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			progress = migrations.UserTopicProgress{
				UserID:      user.ID,
				TopicID:     topic.ID,
				Status:      input.Status,
				Notes:       input.Notes,
				CompletedAt: completedAt,
			}
			if createErr := ctrl.DB.WithContext(konteks.Request.Context()).Create(&progress).Error; createErr != nil {
				response.ServerError(konteks, createErr)
				return
			}
		} else {
			response.ServerError(konteks, err)
			return
		}
	} else {
		progress.Status = input.Status
		if input.Notes != "" {
			progress.Notes = input.Notes
		}
		progress.CompletedAt = completedAt
		if saveErr := ctrl.DB.WithContext(konteks.Request.Context()).Save(&progress).Error; saveErr != nil {
			response.ServerError(konteks, saveErr)
			return
		}
	}

	response.OK(konteks, "progress topik berhasil diperbarui", progress)
}
