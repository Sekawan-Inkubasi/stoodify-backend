package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

func TestUpdateMe_EmailChangeRequiresReverification(t *testing.T) {
	db := setupAITestDB(t)
	gin.SetMode(gin.TestMode)

	user := migrations.User{
		Name:          "Siswa Lama",
		Email:         "lama@example.com",
		PasswordHash:  "hash",
		EmailVerified: true,
	}
	require.NoError(t, db.Create(&user).Error)

	m := &mailerUji{}
	userCtrl := NewUserController(db)
	userCtrl.Mailer = m

	r := gin.New()
	r.Use(func(c *gin.Context) {
		var fresh migrations.User
		require.NoError(t, db.First(&fresh, user.ID).Error)
		c.Set(middlewares.ContextUserKey, fresh)
		c.Next()
	})
	r.PATCH("/api/users/me", userCtrl.UpdateMe)

	// Mengirim email yang sama tidak mereset email_verified.
	wSame := kirim(r, http.MethodPatch, "/api/users/me", `{"email":"lama@example.com"}`)
	require.Equal(t, http.StatusOK, wSame.Code, wSame.Body.String())
	var checkSame migrations.User
	require.NoError(t, db.First(&checkSame, user.ID).Error)
	assert.True(t, checkSame.EmailVerified)

	// Mengubah ke email baru mereset email_verified=false dan mengirim token verifikasi.
	wNew := kirim(r, http.MethodPatch, "/api/users/me", `{"email":"baru@example.com"}`)
	require.Equal(t, http.StatusOK, wNew.Code, wNew.Body.String())

	var checkNew migrations.User
	require.NoError(t, db.First(&checkNew, user.ID).Error)
	assert.Equal(t, "baru@example.com", checkNew.Email)
	assert.False(t, checkNew.EmailVerified, "mengubah email harus mereset email_verified menjadi false")
	require.NotEmpty(t, tungguToken(t, m), "harus mengirim email verifikasi ke alamat baru")
}

func TestSubjectAndTaskRegressions(t *testing.T) {
	db := setupAITestDB(t)
	require.NoError(t, db.AutoMigrate(&migrations.Subtask{}))
	gin.SetMode(gin.TestMode)

	user := migrations.User{Name: "Siswa Tugas", Email: "tugas@example.com", PasswordHash: "x"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&migrations.UserPreference{
		UserID: user.ID, SessionDuration: 60, MaxSessionsPerDay: 3,
	}).Error)

	subjCtrl := NewSubjectController(db)
	taskCtrl := NewTaskController(db)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middlewares.ContextUserKey, user)
		c.Next()
	})
	r.POST("/subjects", subjCtrl.Create)
	r.POST("/tasks", taskCtrl.Create)
	r.GET("/tasks", taskCtrl.GetAll)
	r.GET("/tasks/upcoming", taskCtrl.GetUpcoming)
	r.PATCH("/tasks/:id/progress", taskCtrl.UpdateProgress)
	r.PATCH("/tasks/:id/status", taskCtrl.UpdateStatus)

	// 1. Subject tanpa difficulty harus default ke 3 (memenuhi CHECK difficulty >= 1).
	wSubj := kirim(r, http.MethodPost, "/subjects", `{"name":"Fisika"}`)
	require.Equal(t, http.StatusCreated, wSubj.Code, wSubj.Body.String())
	var subjResp struct {
		Data migrations.Subject `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wSubj.Body.Bytes(), &subjResp))
	assert.Equal(t, 3, subjResp.Data.Difficulty)

	// 2. Task.Create menyimpan material_name dan menghitung priority_score.
	d1 := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	d2 := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	body1 := `{"subject_id":` + itoa(subjResp.Data.ID) + `,"title":"PR 1","material_name":"Hukum Newton","deadline":"` + d1 + `","difficulty":4}`
	body2 := `{"subject_id":` + itoa(subjResp.Data.ID) + `,"title":"PR 2","material_name":"Kinematika","deadline":"` + d2 + `","difficulty":2}`

	wT1 := kirim(r, http.MethodPost, "/tasks", body1)
	require.Equal(t, http.StatusCreated, wT1.Code, wT1.Body.String())
	var t1Resp struct {
		Data migrations.Task `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wT1.Body.Bytes(), &t1Resp))
	assert.Equal(t, "Hukum Newton", t1Resp.Data.MaterialName)
	assert.Greater(t, t1Resp.Data.PriorityScore, 0.0)

	wT2 := kirim(r, http.MethodPost, "/tasks", body2)
	require.Equal(t, http.StatusCreated, wT2.Code, wT2.Body.String())

	// 3. GetAll priority_breakdown.total_score konsisten dengan priority_score tersimpan.
	wList := kirim(r, http.MethodGet, "/tasks", "")
	require.Equal(t, http.StatusOK, wList.Code)
	var listResp struct {
		Data []TaskResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wList.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 2)
	assert.InDelta(t, listResp.Data[0].PriorityScore, listResp.Data[0].PriorityBreakdown.TotalScore, 0.05)

	// 4. GetUpcoming mengurutkan deadline ASC (terdekat dulu).
	wUp := kirim(r, http.MethodGet, "/tasks/upcoming", "")
	require.Equal(t, http.StatusOK, wUp.Code)
	var upResp struct {
		Data []migrations.Task `json:"data"`
	}
	require.NoError(t, json.Unmarshal(wUp.Body.Bytes(), &upResp))
	require.Len(t, upResp.Data, 2)
	assert.Equal(t, "PR 1", upResp.Data[0].Title, "upcoming harus mengurutkan deadline terdekat dulu (ASC)")

	// 5. UpdateProgress menerima 0 (tidak ditolak validator required).
	wP50 := kirim(r, http.MethodPatch, "/tasks/"+itoa(t1Resp.Data.ID)+"/progress", `{"progress":50}`)
	require.Equal(t, http.StatusOK, wP50.Code, wP50.Body.String())
	wP0 := kirim(r, http.MethodPatch, "/tasks/"+itoa(t1Resp.Data.ID)+"/progress", `{"progress":0}`)
	require.Equal(t, http.StatusOK, wP0.Code, wP0.Body.String())
	var afterP0 migrations.Task
	require.NoError(t, db.First(&afterP0, t1Resp.Data.ID).Error)
	assert.Equal(t, 0, afterP0.Progress)

	// 6. UpdateStatus ke done menghitung ulang priority_score menjadi 0.
	wDone := kirim(r, http.MethodPatch, "/tasks/"+itoa(t1Resp.Data.ID)+"/status", `{"status":"done"}`)
	require.Equal(t, http.StatusOK, wDone.Code, wDone.Body.String())
	var afterDone migrations.Task
	require.NoError(t, db.First(&afterDone, t1Resp.Data.ID).Error)
	assert.Equal(t, 100, afterDone.Progress)
	assert.Equal(t, 0.0, afterDone.PriorityScore)

	// Pastikan body tanpa progress tetap ditolak 400.
	reqMissing := httptest.NewRequest(http.MethodPatch, "/tasks/"+itoa(t1Resp.Data.ID)+"/progress", strings.NewReader(`{}`))
	reqMissing.Header.Set("Content-Type", "application/json")
	wMissing := httptest.NewRecorder()
	r.ServeHTTP(wMissing, reqMissing)
	assert.Equal(t, http.StatusBadRequest, wMissing.Code)
}
