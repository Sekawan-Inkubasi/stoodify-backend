package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stoodify_backend/config"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

func setupCurriculumTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(
		&migrations.User{},
		&migrations.Subject{},
		&migrations.CurriculumTopic{},
		&migrations.UserTopicProgress{},
	)
	assert.NoError(t, err)
	return db
}

func TestCurriculumController(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupCurriculumTestDB(t)
	config.DB = db
	ctrl := NewCurriculumController(db)

	// Seed User & Subject
	user := migrations.User{
		Name:         "Student Test",
		Email:        "student1@example.com",
		PasswordHash: "hashed_secret123",
		Role:         migrations.RoleStudent,
	}
	db.Create(&user)

	subject := migrations.Subject{
		UserID:     user.ID,
		Name:       "Matematika Peminatan",
		Color:      "#2563EB",
		Difficulty: 3,
	}
	db.Create(&subject)

	r := gin.New()
	// Mock auth middleware inject currentUser
	r.Use(func(c *gin.Context) {
			c.Set(middlewares.ContextUserKey, user)
		c.Next()
	})

	r.GET("/api/curriculum/roadmap/:subjectId", ctrl.GetRoadmapBySubject)
	r.POST("/api/curriculum/topics", ctrl.CreateTopic)
	r.POST("/api/curriculum/topics/:topicId/progress", ctrl.UpdateProgress)

	t.Run("Create Curriculum Topic", func(t *testing.T) {
		body := map[string]interface{}{
			"subject_id":        subject.ID,
			"title":             "Turunan Fungsi Aljabar",
			"description":       "Konsep dasar limit diferensial dan rumus turunan",
			"order":             1,
			"grade":             11,
			"major":             "IPA",
			"term":              1,
			"estimated_minutes": 90,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/curriculum/topics", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Get Roadmap & Update Topic Progress", func(t *testing.T) {
		var topic migrations.CurriculumTopic
		db.First(&topic, "subject_id = ?", subject.ID)
		assert.NotZero(t, topic.ID)

		// Update progress
		progressBody := map[string]interface{}	{
			"status": "mastered",
			"notes":  "Paham rumus dasar turunan",
		}
		jsonProgress, _ := json.Marshal(progressBody)

		reqProgress := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/curriculum/topics/%d/progress", topic.ID), bytes.NewBuffer(jsonProgress))
		reqProgress.Header.Set("Content-Type", "application/json")
		wProgress := httptest.NewRecorder()
		r.ServeHTTP(wProgress, reqProgress)

		assert.Equal(t, http.StatusOK, wProgress.Code)

		// Get roadmap summary
		reqRoadmap := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/curriculum/roadmap/%d", subject.ID), nil)
		wRoadmap := httptest.NewRecorder()
		r.ServeHTTP(wRoadmap, reqRoadmap)

		assert.Equal(t, http.StatusOK, wRoadmap.Code)
		assert.Contains(t, wRoadmap.Body.String(), "Turunan Fungsi Aljabar")
		assert.Contains(t, wRoadmap.Body.String(), "mastered")
		assert.Contains(t, wRoadmap.Body.String(), "completion_rate")
	})
}
