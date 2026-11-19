package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"

	"stoodify_backend/config"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

func configDB() *gorm.DB { return config.DB }

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

// routerParsial membuat router minimal dengan user sudah terautentikasi,
// dipakai untuk menguji PATCH parsial tanpa middleware JWT.
func routerParsial(user migrations.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(k *gin.Context) {
		k.Set(middlewares.ContextUserKey, user)
		k.Next()
	})

	activity := NewActivityController(configDB())
	r.PATCH("/activities/:id", activity.Update)
	r.POST("/activities", activity.Create)

	task := NewTaskController(configDB())
	r.PATCH("/tasks/:id", task.Update)

	schedule := NewScheduleController(configDB())
	r.PATCH("/schedules/:id", schedule.Update)

	subject := NewSubjectController(configDB())
	r.PATCH("/subjects/:id", subject.Update)

	return r
}

func kirim(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setupUserDenganDB(t *testing.T, email string) (*gin.Engine, migrations.User, *migrations.Subject) {
	t.Helper()
	db := setupAITestDB(t)

	user := migrations.User{Name: "Siswa Patch", Email: email, PasswordHash: "x"}
	require.NoError(t, db.Create(&user).Error)
	subject := migrations.Subject{UserID: user.ID, Name: "Matematika", Difficulty: 3}
	require.NoError(t, db.Create(&subject).Error)

	return routerParsial(user), user, &subject
}

func TestActivityPatchParsialMenjagaFieldLain(t *testing.T) {
	r, _, _ := setupUserDenganDB(t, "patch-activity@example.com")

	awal := `{"title":"Latihan Robotik","start_time":"15:30","end_time":"17:30","recurrence":"weekly","day_of_week":"Rabu"}`
	w := kirim(r, http.MethodPost, "/activities", awal)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var dibuat migrations.Activity
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &struct {
		Data *migrations.Activity `json:"data"`
	}{Data: &dibuat}))
	assert.Equal(t, "Wednesday", dibuat.DayOfWeek, "day_of_week harus dinormalkan dan disimpan")

	w = kirim(r, http.MethodPatch, "/activities/"+itoa(dibuat.ID), `{"title":"Latihan Robotik Persiapan Lomba"}`)
	require.Equal(t, http.StatusOK, w.Code, "PATCH parsial tidak boleh 400: %s", w.Body.String())

	var sesudah migrations.Activity
	require.NoError(t, configDB().First(&sesudah, dibuat.ID).Error)
	assert.Equal(t, "Latihan Robotik Persiapan Lomba", sesudah.Title)
	assert.Equal(t, "15:30", sesudah.StartTime)
	assert.Equal(t, "17:30", sesudah.EndTime)
	assert.Equal(t, "weekly", sesudah.Recurrence)
	assert.Equal(t, "Wednesday", sesudah.DayOfWeek)
}

func TestActivityPatchMenolakHariTidakValid(t *testing.T) {
	r, _, _ := setupUserDenganDB(t, "patch-activity-day@example.com")

	w := kirim(r, http.MethodPost, "/activities", `{"title":"Les","start_time":"16:00","end_time":"17:00"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var dibuat migrations.Activity
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &struct {
		Data *migrations.Activity `json:"data"`
	}{Data: &dibuat}))

	w = kirim(r, http.MethodPatch, "/activities/"+itoa(dibuat.ID), `{"day_of_week":"Hari Libur"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTaskPatchParsialMenjagaDeadline(t *testing.T) {
	r, user, subject := setupUserDenganDB(t, "patch-task@example.com")
	deadline := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)

	task := migrations.Task{
		UserID: user.ID, SubjectID: subject.ID, Title: "Tugas Awal",
		Deadline: deadline, Difficulty: 3, EstimatedDuration: 60, Status: migrations.StatusPending,
	}
	require.NoError(t, configDB().Create(&task).Error)

	w := kirim(r, http.MethodPatch, "/tasks/"+itoa(task.ID), `{"title":"Tugas Revisi"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.Task
	require.NoError(t, configDB().First(&sesudah, task.ID).Error)
	assert.Equal(t, "Tugas Revisi", sesudah.Title)
	assert.Equal(t, 3, sesudah.Difficulty)
	assert.Equal(t, 60, sesudah.EstimatedDuration)
	assert.WithinDuration(t, deadline, sesudah.Deadline, time.Second)
}

func TestSchedulePatchParsialMenjagaWaktu(t *testing.T) {
	r, user, subject := setupUserDenganDB(t, "patch-schedule@example.com")

	schedule := migrations.Schedule{
		UserID: user.ID, SubjectID: subject.ID, DayOfWeek: "Monday",
		StartTime: "07:00", EndTime: "08:30",
	}
	require.NoError(t, configDB().Create(&schedule).Error)

	w := kirim(r, http.MethodPatch, "/schedules/"+itoa(schedule.ID), `{"room":"Lab 2"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.Schedule
	require.NoError(t, configDB().First(&sesudah, schedule.ID).Error)
	assert.Equal(t, "Lab 2", sesudah.Room)
	assert.Equal(t, "Monday", sesudah.DayOfWeek)
	assert.Equal(t, "07:00", sesudah.StartTime)
	assert.Equal(t, "08:30", sesudah.EndTime)
}

func TestSubjectPatchParsialMenjagaNama(t *testing.T) {
	r, _, subject := setupUserDenganDB(t, "patch-subject@example.com")

	w := kirim(r, http.MethodPatch, "/subjects/"+itoa(subject.ID), `{"color":"#22c55e"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var sesudah migrations.Subject
	require.NoError(t, configDB().First(&sesudah, subject.ID).Error)
	assert.Equal(t, "#22c55e", sesudah.Color)
	assert.Equal(t, "Matematika", sesudah.Name)
}
