package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/params"
	"stoodify_backend/internal/response"
	"stoodify_backend/migrations"
)

type AIController struct {
	DB *gorm.DB
}

func NewAIController(db *gorm.DB) *AIController {
	return &AIController{DB: db}
}

type GenerateScheduleInput struct {
	Date string `json:"date" binding:"omitempty,len=10"` // YYYY-MM-DD, default hari ini
}

type GeneratePredictionInput struct {
	SubjectID uint `json:"subject_id" binding:"required"`
}

type FeedbackInput struct {
	Feedback string `json:"feedback" binding:"required,oneof=accurate inaccurate"`
}

// slotRekomendasi: satu sesi belajar usulan AI.
type slotRekomendasi struct {
	TaskID          uint   `json:"task_id"`
	RecommendedDate string `json:"recommended_date"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Duration        int    `json:"duration"`
	TargetProgress  int    `json:"target_progress"`
	Priority        string `json:"priority"`
	Reason          string `json:"reason"`
}

// GenerateSchedule memanggil AI Spec 1; kalau AI gagal/timeout, pakai fallback rule-based.
func (ctrl *AIController) GenerateSchedule(konteks *gin.Context) {
	user := currentUser(konteks)
	var input GenerateScheduleInput

	if err := konteks.ShouldBindJSON(&input); err != nil && err.Error() != "EOF" {
		response.ValidationError(konteks, err)
		return
	}

	targetDate := time.Now()
	if input.Date != "" {
		parsed, err := time.Parse("2006-01-02", input.Date)
		if err != nil {
			response.UnprocessableEntity(konteks, "format date tidak valid", map[string]string{
				"date": "harus YYYY-MM-DD",
			})
			return
		}
		targetDate = parsed
	}

	var tasks []migrations.Task
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ? AND status <> ?", user.ID, migrations.StatusDone).
		Order("priority_score DESC, deadline ASC").Find(&tasks).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	if len(tasks) == 0 {
		response.OK(konteks, "tidak ada tugas aktif untuk dijadwalkan", []migrations.StudySession{})
		return
	}

	var pref migrations.UserPreference
	ctrl.DB.Where("user_id = ?", user.ID).First(&pref)
	if pref.StartTime == "" {
		pref.StartTime = "15:00"
	}
	if pref.EndTime == "" {
		pref.EndTime = "21:00"
	}
	if pref.SessionDuration <= 0 {
		pref.SessionDuration = 60
	}

	slots, source := ctrl.panggilAIScheduler(konteks, user.ID, targetDate, tasks, pref)
	if source == "" {
		response.Fail(konteks, 502, "layanan AI tidak dapat dihubungi, silakan coba lagi", nil)
		return
	}

	var sessions []migrations.StudySession
	for _, slot := range slots {
		taskID := slot.TaskID
		if _, ok := findOwnedTask(user.ID, taskID); !ok {
			continue // AI mengembalikan task yang bukan milik user: abaikan
		}
		duration := slot.Duration
		if duration <= 0 {
			duration = pref.SessionDuration
		}
		startTime := slot.StartTime
		if hhmmMinutes(startTime) < 0 {
			continue
		}
		sessionDate := targetDate
		if slot.RecommendedDate != "" {
			if parsed, err := time.Parse("2006-01-02", slot.RecommendedDate); err == nil {
				sessionDate = parsed
			}
		}
		sessions = append(sessions, migrations.StudySession{
			UserID:         user.ID,
			TaskID:         taskID,
			SessionDate:    sessionDate,
			StartTime:      startTime,
			Duration:       duration,
			TargetProgress: slot.TargetProgress,
			Status:         migrations.SessionPlanned,
			AIGenerated:    true,
			Reason:         slot.Reason,
		})
	}

	if len(sessions) == 0 {
		response.Fail(konteks, 502, "AI tidak memberikan slot yang bisa dipakai", nil)
		return
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&sessions).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.Created(konteks, "rekomendasi sesi belajar dibuat ("+source+")", sessions)
}

// panggilAIScheduler: coba AI Spec 1 dulu, fallback ke time-slot finder lokal.
// Mengembalikan (slots, sumber); sumber kosong = AI gagal DAN fallback dimatikan.
func (ctrl *AIController) panggilAIScheduler(konteks *gin.Context, userID uint, date time.Time, tasks []migrations.Task, pref migrations.UserPreference) ([]slotRekomendasi, string) {
	url := os.Getenv("AI_SCHEDULER_URL")
	if url != "" {
		payload := gin.H{
			"user_id": userID,
			"date":    date.Format("2006-01-02"),
			"preferences": gin.H{
				"start_time":       pref.StartTime,
				"end_time":         pref.EndTime,
				"session_duration": pref.SessionDuration,
				"active_days":      pref.ActiveDays,
			},
			"tasks": tasks,
		}
		var result struct {
			Sessions []slotRekomendasi `json:"sessions"`
		}
		if err := postJSON(konteks.Request.Context(), url, payload, &result); err == nil && len(result.Sessions) > 0 {
			return result.Sessions, "ai"
		}
	}

	if os.Getenv("AI_FALLBACK_ENABLED") == "false" {
		return nil, ""
	}
	return fallbackSlots(date, tasks, pref), "fallback rule-based"
}

// fallbackSlots membagi tugas terprioritas ke slot kosong antara start-end preferensi.
func fallbackSlots(date time.Time, tasks []migrations.Task, pref migrations.UserPreference) []slotRekomendasi {
	dayStart := hhmmMinutes(pref.StartTime)
	dayEnd := hhmmMinutes(pref.EndTime)
	if dayStart < 0 || dayEnd < 0 || dayStart >= dayEnd {
		return nil
	}

	ordered := make([]migrations.Task, len(tasks))
	copy(ordered, tasks)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].PriorityScore > ordered[j].PriorityScore
	})

	duration := pref.SessionDuration
	if duration <= 0 {
		duration = 60
	}

	var slots []slotRekomendasi
	cursor := dayStart
	for _, task := range ordered {
		if cursor+duration > dayEnd {
			break
		}
		remaining := 100 - task.Progress
		target := 30
		if remaining < target {
			target = remaining
		}
		urgency := "low"
		if task.PriorityScore >= 70 {
			urgency = "high"
		} else if task.PriorityScore >= 40 {
			urgency = "medium"
		}

		slots = append(slots, slotRekomendasi{
			TaskID:          task.ID,
			RecommendedDate: date.Format("2006-01-02"),
			StartTime:       minutesToHHMM(cursor),
			EndTime:         minutesToHHMM(cursor + duration),
			Duration:        duration,
			TargetProgress:  target,
			Priority:        urgency,
			Reason: fmt.Sprintf("Skor prioritas %.0f, deadline %s, progres baru %d%%",
				task.PriorityScore, task.Deadline.Format("02 Jan 2006"), task.Progress),
		})
		cursor += duration
	}
	return slots
}

// AcceptRecommendation: sesi AI disetujui, tetap di kalender.
func (ctrl *AIController) AcceptRecommendation(konteks *gin.Context) {
	ctrl.setSessionStatus(konteks, migrations.SessionPlanned, "rekomendasi diterima")
}

// RejectRecommendation: sesi AI ditolak, status jadi skipped.
func (ctrl *AIController) RejectRecommendation(konteks *gin.Context) {
	ctrl.setSessionStatus(konteks, migrations.SessionSkipped, "rekomendasi ditolak")
}

func (ctrl *AIController) setSessionStatus(konteks *gin.Context, status, message string) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	result := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.StudySession{}).
		Where("id = ? AND user_id = ?", id, user.ID).
		Update("status", status)
	if result.Error != nil {
		response.ServerError(konteks, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(konteks, "sesi tidak ditemukan")
		return
	}

	var session migrations.StudySession
	ctrl.DB.WithContext(konteks.Request.Context()).Preload("Task").First(&session, id)
	response.OK(konteks, message, session)
}

func (ctrl *AIController) GetScheduleRecommendations(konteks *gin.Context) {
	user := currentUser(konteks)

	var sessions []migrations.StudySession
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Task").
		Where("user_id = ? AND ai_generated = ?", user.ID, true).
		Order("session_date ASC, start_time ASC").
		Find(&sessions).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "rekomendasi sesi belajar", sessions)
}

// Reschedule: susun ulang sesi yang belum selesai untuk hari ini.
func (ctrl *AIController) Reschedule(konteks *gin.Context) {
	user := currentUser(konteks)

	var pending []migrations.StudySession
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ? AND status = ?", user.ID, migrations.SessionPlanned).
		Find(&pending).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	if len(pending) == 0 {
		response.OK(konteks, "tidak ada sesi yang perlu disusun ulang", nil)
		return
	}

	var pref migrations.UserPreference
	ctrl.DB.Where("user_id = ?", user.ID).First(&pref)
	if pref.StartTime == "" {
		pref.StartTime = "15:00"
	}
	if pref.EndTime == "" {
		pref.EndTime = "21:00"
	}
	if pref.SessionDuration <= 0 {
		pref.SessionDuration = 60
	}

	dayStart := hhmmMinutes(pref.StartTime)
	dayEnd := hhmmMinutes(pref.EndTime)
	if dayStart < 0 || dayEnd < 0 || dayStart >= dayEnd {
		response.UnprocessableEntity(konteks, "preferensi waktu belajar belum valid", nil)
		return
	}

	today := time.Now()
	cursor := dayStart
	var updated []migrations.StudySession
	for i := range pending {
		if cursor+pref.SessionDuration > dayEnd {
			break
		}
		pending[i].SessionDate = today
		pending[i].StartTime = minutesToHHMM(cursor)
		pending[i].Duration = pref.SessionDuration
		if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&pending[i]).Error; err != nil {
			response.ServerError(konteks, err)
			return
		}
		updated = append(updated, pending[i])
		cursor += pref.SessionDuration
	}

	if len(updated) == 0 {
		response.Fail(konteks, 422, "waktu belajar tidak cukup untuk sesi yang tersisa", nil)
		return
	}
	response.OK(konteks, "jadwal sesi disusun ulang", updated)
}

// GeneratePrediction memanggil AI Spec 2 dan menyimpan hasilnya.
func (ctrl *AIController) GeneratePrediction(konteks *gin.Context) {
	user := currentUser(konteks)
	var input GeneratePredictionInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	subject, found := findOwnedSubject(user.ID, input.SubjectID)
	if !found {
		response.NotFound(konteks, "mata pelajaran tidak ditemukan")
		return
	}

	var history []migrations.Task
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ? AND subject_id = ?", user.ID, subject.ID).
		Order("deadline DESC").Limit(10).Find(&history).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	prediction, source := ctrl.panggilAIPredictor(konteks, subject, history)
	if source == "" {
		response.Fail(konteks, 502, "layanan AI tidak dapat dihubungi, silakan coba lagi", nil)
		return
	}

	record := migrations.MaterialPrediction{
		UserID:            user.ID,
		SubjectID:         subject.ID,
		PredictedMaterial: prediction.PredictedMaterial,
		Confidence:        prediction.Confidence,
		Reason:            prediction.Reason,
		Prerequisites:     prediction.Prerequisites,
		Preparation:       prediction.Preparation,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&record).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.Created(konteks, "prediksi materi dibuat ("+source+")", record)
}

type hasilPrediksi struct {
	PredictedMaterial string  `json:"predicted_topic"`
	Confidence        float64 `json:"confidence"`
	Reason            string  `json:"reason"`
	Prerequisites     string  `json:"prerequisites"`
	Preparation       string  `json:"preparation_suggestion"`
}

func (ctrl *AIController) panggilAIPredictor(konteks *gin.Context, subject migrations.Subject, history []migrations.Task) (hasilPrediksi, string) {
	url := os.Getenv("AI_PREDICTOR_URL")
	if url != "" {
		payload := gin.H{"subject": subject.Name, "difficulty": subject.Difficulty, "history": history}
		var result hasilPrediksi
		if err := postJSON(konteks.Request.Context(), url, payload, &result); err == nil && result.PredictedMaterial != "" {
			return result, "ai"
		}
	}

	if os.Getenv("AI_FALLBACK_ENABLED") == "false" {
		return hasilPrediksi{}, ""
	}
	return fallbackPrediction(subject, history), "fallback rule-based"
}

// fallbackPrediction: tanpa LLM, prediksi memakai judul tugas terakhir mapel itu.
func fallbackPrediction(subject migrations.Subject, history []migrations.Task) hasilPrediksi {
	reason := "Belum ada riwayat tugas pada mata pelajaran ini"
	topic := "Materi lanjutan " + subject.Name
	if len(history) > 0 {
		reason = fmt.Sprintf("Berdasarkan %d tugas terakhir: %s", len(history), history[0].Title)
		topic = "Lanjutan dari " + history[0].Title
	}
	return hasilPrediksi{
		PredictedMaterial: topic,
		Confidence:        35,
		Reason:            reason,
		Prerequisites:     subject.Name + " dasar",
		Preparation:       "Ulangi materi " + subject.Name + " sebelumnya sebelum kelas berikutnya",
	}
}

func (ctrl *AIController) GetPrediction(konteks *gin.Context) {
	user := currentUser(konteks)
	subjectID, ok := params.IDParam(konteks, "subjectId")
	if !ok {
		return
	}

	var predictions []migrations.MaterialPrediction
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Preload("Subject").
		Where("user_id = ? AND subject_id = ?", user.ID, subjectID).
		Order("created_at DESC").
		Find(&predictions).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "prediksi materi", predictions)
}

func (ctrl *AIController) SubmitFeedback(konteks *gin.Context) {
	user := currentUser(konteks)
	id, ok := params.IDParam(konteks, "id")
	if !ok {
		return
	}

	var input FeedbackInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	result := ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.MaterialPrediction{}).
		Where("id = ? AND user_id = ?", id, user.ID).
		Update("user_feedback", input.Feedback)
	if result.Error != nil {
		response.ServerError(konteks, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		response.NotFound(konteks, "prediksi tidak ditemukan")
		return
	}

	var prediction migrations.MaterialPrediction
	ctrl.DB.WithContext(konteks.Request.Context()).First(&prediction, id)
	response.OK(konteks, "umpan balik disimpan", prediction)
}

// postJSON mengirim request dengan timeout 15 detik sesuai PRD §Non-Fungsional.
func postJSON(ctx context.Context, url string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if key := os.Getenv("AI_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("layanan AI mengembalikan status " + res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func minutesToHHMM(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}
