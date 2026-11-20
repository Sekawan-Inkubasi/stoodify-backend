package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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

// ==================== Penjadwalan otomatis & manual ====================

// jedaSusunUlangManual: batas tombol "Susun ulang" per user.
const jedaSusunUlangManual = 10 * time.Minute

var susunUlangManual = struct {
	mu       sync.Mutex
	terakhir map[uint]time.Time
}{terakhir: make(map[uint]time.Time)}

// bolehSusunUlangManual mengembalikan sisa waktu tunggu dan status boleh/tidak.
func bolehSusunUlangManual(userID uint) (time.Duration, bool) {
	susunUlangManual.mu.Lock()
	defer susunUlangManual.mu.Unlock()
	terakhir, ada := susunUlangManual.terakhir[userID]
	if !ada {
		return 0, true
	}
	sisa := jedaSusunUlangManual - time.Since(terakhir)
	if sisa <= 0 {
		return 0, true
	}
	return sisa, false
}

func catatSusunUlangManual(userID uint) {
	susunUlangManual.mu.Lock()
	defer susunUlangManual.mu.Unlock()
	susunUlangManual.terakhir[userID] = time.Now()
}

// konteksJadwal: seluruh data yang dikirim ke service AI.
type konteksJadwal struct {
	User       migrations.User
	Tanggal    time.Time
	Tasks      []migrations.Task
	Pref       migrations.UserPreference
	Schedules  []migrations.Schedule
	Activities []migrations.Activity
	Existing   []migrations.StudySession
}

// hasilRegenerasi: hasil satu kali penyusunan jadwal.
type hasilRegenerasi struct {
	Sessions     []migrations.StudySession
	Meta         aiSchedulerMeta
	TidakBerubah bool
	TanpaTugas   bool
}

// GenerateSchedule: jalur MANUAL (tombol "Susun ulang"), dibatasi 1x per 10 menit.
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

	if sisa, boleh := bolehSusunUlangManual(user.ID); !boleh {
		menit := int(sisa.Minutes()) + 1
		konteks.Header("Retry-After", strconv.Itoa(int(sisa.Seconds())+1))
		response.Fail(konteks, http.StatusTooManyRequests,
			fmt.Sprintf("Jadwal kamu baru saja diperbarui. Coba lagi dalam %d menit, atau geser sendiri sesi yang mau diubah.", menit), nil)
		return
	}

	hasil, err := ctrl.regenerasi(konteks.Request.Context(), user, targetDate)
	if err != nil {
		teruskanKegagalanAI(konteks, err)
		return
	}
	catatSusunUlangManual(user.ID)

	switch {
	case hasil.TanpaTugas:
		response.OK(konteks, "tidak ada tugas aktif untuk dijadwalkan", []migrations.StudySession{})
	case hasil.TidakBerubah:
		response.OK(konteks, "Jadwal kamu sudah paling pas — belum ada yang perlu disusun ulang.", hasil.Sessions)
	default:
		response.Created(konteks, "jadwal belajar disusun ulang", hasil.Sessions)
	}
}

// RegenerasiOtomatis dijalankan planner (background) setiap ada perubahan konteks.
// Gagal = diam: jadwal lama dibiarkan, cukup dicatat di log.
func (ctrl *AIController) RegenerasiOtomatis(userID uint) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var user migrations.User
	if err := ctrl.DB.WithContext(ctx).First(&user, userID).Error; err != nil {
		log.Printf("planner: user %d tidak ditemukan: %v", userID, err)
		return
	}
	// Akun Google yang belum onboarding belum punya sekolah/kelas/jurusan:
	// jangan buat jadwal dari data kosong (dan jangan spam log error).
	if !user.ProfileCompleted {
		log.Printf("planner: user %d dilewati (profil belum lengkap)", userID)
		return
	}

	mulai := time.Now()
	hasil, err := ctrl.regenerasi(ctx, user, time.Now())
	if err != nil {
		log.Printf("planner: regenerasi user %d gagal (jadwal lama dipertahankan): %v", userID, err)
		return
	}
	if hasil.TanpaTugas {
		return
	}
	log.Printf("planner: user=%d sesi=%d source=%s tidak_berubah=%t ms=%d",
		userID, len(hasil.Sessions), hasil.Meta.Source, hasil.TidakBerubah, time.Since(mulai).Milliseconds())
}

// regenerasi: bangun konteks -> panggil AI -> susun sesi -> simpan (transaksi).
func (ctrl *AIController) regenerasi(ctx context.Context, user migrations.User, tanggal time.Time) (hasilRegenerasi, error) {
	k, err := ctrl.bangunKonteks(ctx, user, tanggal)
	if err != nil {
		return hasilRegenerasi{}, err
	}
	if len(k.Tasks) == 0 {
		return hasilRegenerasi{TanpaTugas: true}, nil
	}

	slots, meta, err := ctrl.panggilAIScheduler(ctx, user.ID, tanggal, k)
	if err != nil {
		return hasilRegenerasi{}, err
	}
	if len(meta.Warnings) > 0 {
		log.Printf("ai: engine mengirim %d peringatan: %s", len(meta.Warnings), strings.Join(meta.Warnings, " | "))
	}

	sessions := ctrl.susunSesi(k, slots, meta)
	if len(sessions) == 0 {
		return hasilRegenerasi{}, &aiResponseError{
			StatusCode: http.StatusBadGateway,
			Message:    "AI tidak memberikan slot yang bisa dipakai",
		}
	}

	berubah, err := ctrl.simpanRegenerasi(ctx, k, sessions, meta)
	if err != nil {
		return hasilRegenerasi{}, err
	}
	return hasilRegenerasi{Sessions: sessions, Meta: meta, TidakBerubah: !berubah}, nil
}

// bangunKonteks membaca seluruh data yang dibutuhkan engine (read-only).
func (ctrl *AIController) bangunKonteks(ctx context.Context, user migrations.User, tanggal time.Time) (konteksJadwal, error) {
	db := ctrl.DB.WithContext(ctx)
	k := konteksJadwal{User: user, Tanggal: tanggal}

	if err := db.Preload("Subject").
		Where("user_id = ? AND status <> ?", user.ID, migrations.StatusDone).
		Order("priority_score DESC, deadline ASC").Find(&k.Tasks).Error; err != nil {
		return k, err
	}

	if err := db.Where("user_id = ?", user.ID).First(&k.Pref).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return k, err
	}
	if k.Pref.StartTime == "" {
		k.Pref.StartTime = "15:00"
	}
	if k.Pref.EndTime == "" {
		k.Pref.EndTime = "21:00"
	}
	if k.Pref.SessionDuration <= 0 {
		k.Pref.SessionDuration = 60
	}

	if err := db.Preload("Subject").Where("user_id = ?", user.ID).Find(&k.Schedules).Error; err != nil {
		return k, err
	}
	if err := db.Where("user_id = ?", user.ID).Find(&k.Activities).Error; err != nil {
		return k, err
	}
	if err := db.Where("user_id = ? AND session_date >= ? AND status = ?",
		user.ID, tanggal.Format("2006-01-02"), migrations.SessionPlanned).Find(&k.Existing).Error; err != nil {
		return k, err
	}
	return k, nil
}

// susunSesi mengubah slot dari AI menjadi baris StudySession.
// Slot yang bertabrakan dengan sesi terkunci (pinned / < 24 jam) dibuang agar
// jadwal milik user tidak tergeser walau engine belum mendukung `locked`.
func (ctrl *AIController) susunSesi(k konteksJadwal, slots []slotRekomendasi, meta aiSchedulerMeta) []migrations.StudySession {
	terkunci := ctrl.sesiTerkunci(k)

	sessions := make([]migrations.StudySession, 0, len(slots))
	for _, slot := range slots {
		if _, ok := findOwnedTask(k.User.ID, slot.TaskID); !ok {
			continue
		}
		start := hhmmMinutes(slot.StartTime)
		if start < 0 {
			continue
		}
		duration := slot.Duration
		if duration <= 0 {
			duration = k.Pref.SessionDuration
		}
		tanggal := k.Tanggal
		if slot.RecommendedDate != "" {
			if parsed, err := time.Parse("2006-01-02", slot.RecommendedDate); err == nil {
				tanggal = parsed
			}
		}
		if bentrokDenganTerkunci(terkunci, tanggal.Format("2006-01-02"), start, duration) {
			continue
		}

		sessions = append(sessions, migrations.StudySession{
			UserID:         k.User.ID,
			TaskID:         slot.TaskID,
			SessionDate:    tanggal,
			StartTime:      slot.StartTime,
			Duration:       duration,
			TargetProgress: slot.TargetProgress,
			Status:         migrations.SessionPlanned,
			AIGenerated:    true,
			Reason:         slot.Reason,
			AISource:       meta.Source,
			AIWarnings:     strings.Join(meta.Warnings, " | "),
		})
	}
	return sessions
}

type sesiTerkunciInfo struct {
	Tanggal string
	Mulai   int
	Selesai int
}

func (ctrl *AIController) sesiTerkunci(k konteksJadwal) []sesiTerkunciInfo {
	var hasil []sesiTerkunciInfo
	batas := time.Now().Add(24 * time.Hour)
	for _, s := range k.Existing {
		start := hhmmMinutes(s.StartTime)
		if start < 0 || s.Duration <= 0 {
			continue
		}
		terkunci := !s.AIGenerated || s.SessionDate.Before(batas)
		if !terkunci {
			continue
		}
		hasil = append(hasil, sesiTerkunciInfo{
			Tanggal: s.SessionDate.Format("2006-01-02"),
			Mulai:   start,
			Selesai: start + s.Duration,
		})
	}
	return hasil
}

func bentrokDenganTerkunci(terkunci []sesiTerkunciInfo, tanggal string, mulai, durasi int) bool {
	selesai := mulai + durasi
	for _, t := range terkunci {
		if t.Tanggal != tanggal {
			continue
		}
		if mulai < t.Selesai && selesai > t.Mulai {
			return true
		}
	}
	return false
}

// simpanRegenerasi menulis hasil ke DB dalam satu transaksi:
// snapshot -> hapus sesi AI planned pada tanggal itu -> insert sesi baru.
func (ctrl *AIController) simpanRegenerasi(ctx context.Context, k konteksJadwal, sessions []migrations.StudySession, meta aiSchedulerMeta) (bool, error) {
	if sesiSama(k.Existing, sessions) {
		return false, nil
	}

	versi := int(time.Now().Unix())
	for i := range sessions {
		sessions[i].PlanVersion = versi
	}

	err := ctrl.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Snapshot jadwal saat ini untuk fitur "Urungkan".
		if payload, err := json.Marshal(k.Existing); err == nil {
			if err := tx.Create(&migrations.ScheduleSnapshot{
				UserID:      k.User.ID,
				SessionDate: k.Tanggal,
				Payload:     string(payload),
			}).Error; err != nil {
				return err
			}
		}

		// 2. Hapus hanya sesi AI planned pada tanggal target (rentang hari, aman untuk sqlite/PG).
		awal, akhir := rentangHari(k.Tanggal)
		if err := tx.Where("user_id = ? AND session_date >= ? AND session_date < ? AND status = ? AND ai_generated = ?",
			k.User.ID, awal, akhir, migrations.SessionPlanned, true).
			Delete(&migrations.StudySession{}).Error; err != nil {
			return err
		}

		// 3. Simpan sesi baru.
		return tx.Create(&sessions).Error
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// sesiSama: true kalau jadwal hasil AI identik dengan sesi AI yang sudah ada
// (dipakai untuk pesan "sudah paling pas" tanpa menulis apa pun).
func sesiSama(existing []migrations.StudySession, baru []migrations.StudySession) bool {
	lama := make([]migrations.StudySession, 0, len(existing))
	for _, s := range existing {
		if s.AIGenerated {
			lama = append(lama, s)
		}
	}
	if len(lama) != len(baru) {
		return false
	}
	kunci := func(s migrations.StudySession) string {
		return fmt.Sprintf("%d|%s|%s|%d", s.TaskID, s.SessionDate.Format("2006-01-02"), s.StartTime, s.Duration)
	}
	set := make(map[string]int, len(lama))
	for _, s := range lama {
		set[kunci(s)]++
	}
	for _, s := range baru {
		k := kunci(s)
		if set[k] == 0 {
			return false
		}
		set[k]--
	}
	return true
}

// rentangHari mengembalikan batas awal & akhir (eksklusif) satu hari kalender,
// dipakai untuk query session_date yang bertipe timestamp.
func rentangHari(tanggal time.Time) (time.Time, time.Time) {
	awal := time.Date(tanggal.Year(), tanggal.Month(), tanggal.Day(), 0, 0, 0, 0, tanggal.Location())
	return awal, awal.AddDate(0, 0, 1)
}

// UndoRegenerasi: kembalikan jadwal ke snapshot terakhir (tombol "Urungkan").
func (ctrl *AIController) UndoRegenerasi(konteks *gin.Context) {
	user := currentUser(konteks)

	var snap migrations.ScheduleSnapshot
	if err := ctrl.DB.WithContext(konteks.Request.Context()).
		Where("user_id = ?", user.ID).Order("created_at DESC").First(&snap).Error; err != nil {
		response.NotFound(konteks, "tidak ada jadwal sebelumnya yang bisa dikembalikan")
		return
	}

	var sesiLama []migrations.StudySession
	if err := json.Unmarshal([]byte(snap.Payload), &sesiLama); err != nil {
		response.ServerError(konteks, err)
		return
	}

	var dipulihkan []migrations.StudySession
	err := ctrl.DB.WithContext(konteks.Request.Context()).Transaction(func(tx *gorm.DB) error {
		awal, akhir := rentangHari(snap.SessionDate)
		if err := tx.Where("user_id = ? AND session_date >= ? AND session_date < ? AND status = ? AND ai_generated = ?",
			user.ID, awal, akhir, migrations.SessionPlanned, true).
			Delete(&migrations.StudySession{}).Error; err != nil {
			return err
		}
		for _, s := range sesiLama {
			dipulihkan = append(dipulihkan, migrations.StudySession{
				UserID:         user.ID,
				TaskID:         s.TaskID,
				SessionDate:    s.SessionDate,
				StartTime:      s.StartTime,
				Duration:       s.Duration,
				TargetProgress: s.TargetProgress,
				Status:         s.Status,
				AIGenerated:    s.AIGenerated,
				Reason:         s.Reason,
				AISource:       s.AISource,
				AIWarnings:     s.AIWarnings,
			})
		}
		if len(dipulihkan) == 0 {
			return nil
		}
		if err := tx.Create(&dipulihkan).Error; err != nil {
			return err
		}
		return tx.Delete(&snap).Error
	})
	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "jadwal dikembalikan ke versi sebelumnya", dipulihkan)
}

// aiSchedulerMeta: metadata balikan engine yang diteruskan ke klien, bukan diolah.
type aiSchedulerMeta struct {
	Source         string   `json:"source"`
	Warnings       []string `json:"warnings"`
	FallbackReason string   `json:"fallback_reason"`
}

// zonaWaktu: zona yang dikirim ke engine; ikut env TZ server (Dockerfile: Asia/Jakarta).
func zonaWaktu() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		return tz
	}
	return "Asia/Jakarta"
}

// sesiKirim: sesi existing + penanda `locked` (milik user / terlalu dekat untuk digeser).
type sesiKirim struct {
	migrations.StudySession
	Locked bool `json:"locked"`
}

// panggilAIScheduler memanggil AI Spec 1. Semua kegagalan dikembalikan sebagai error
// agar controller bisa meneruskan status dari service AI; backend tidak punya fallback lokal.
func (ctrl *AIController) panggilAIScheduler(ctx context.Context, userID uint, tanggal time.Time, k konteksJadwal) ([]slotRekomendasi, aiSchedulerMeta, error) {
	url := os.Getenv("AI_SCHEDULER_URL")
	if url == "" {
		return nil, aiSchedulerMeta{}, errors.New("AI_SCHEDULER_URL belum diset")
	}

	pref := k.Pref
	breakDuration := pref.BreakDuration
	if breakDuration <= 0 {
		breakDuration = 15
	}
	maxSessions := pref.MaxSessionsPerDay
	if maxSessions <= 0 {
		maxSessions = 3
	}

	// Schedules dikirim dengan `title` (nama mapel) agar prompt engine lebih jelas.
	jadwal := make([]gin.H, 0, len(k.Schedules))
	for _, s := range k.Schedules {
		title := ""
		if s.Subject != nil {
			title = s.Subject.Name
		}
		jadwal = append(jadwal, gin.H{
			"day_of_week": s.DayOfWeek,
			"start_time":  s.StartTime,
			"end_time":    s.EndTime,
			"title":       title,
			"room":        s.Room,
			"teacher":     s.Teacher,
		})
	}

	batas := time.Now().Add(24 * time.Hour)
	existing := make([]sesiKirim, 0, len(k.Existing))
	for _, s := range k.Existing {
		existing = append(existing, sesiKirim{
			StudySession: s,
			Locked:       !s.AIGenerated || s.SessionDate.Before(batas),
		})
	}

	payload := gin.H{
		"user_id":  userID,
		"date":     tanggal.Format("2006-01-02"),
		"now":      time.Now().Format(time.RFC3339),
		"timezone": zonaWaktu(),
		"preferences": gin.H{
			"start_time":           pref.StartTime,
			"end_time":             pref.EndTime,
			"session_duration":     pref.SessionDuration,
			"break_duration":       breakDuration,
			"max_sessions_per_day": maxSessions,
			"active_days":          pref.ActiveDays,
		},
		"tasks":             k.Tasks,
		"schedules":         jadwal,
		"activities":        k.Activities,
		"existing_sessions": existing,
	}

	var raw json.RawMessage
	if err := postJSON(ctx, url, payload, &raw); err != nil {
		return nil, aiSchedulerMeta{}, err
	}
	slots, meta := parseSchedulerResponse(raw)
	return slots, meta, nil
}

// aiSlotRaw adalah bentuk mentah slot dari service AI. Service AI boleh memakai
// nama field gaya kontrak (`session_date`) maupun gaya internal (`recommended_date`);
// keduanya diterima agar integrasi lintas tim tidak pecah karena beda penamaan.
type aiSlotRaw struct {
	TaskID          uint   `json:"task_id"`
	TaskTitle       string `json:"task_title"`
	RecommendedDate string `json:"recommended_date"`
	SessionDate     string `json:"session_date"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Duration        int    `json:"duration"`
	TargetProgress  int    `json:"target_progress"`
	Priority        string `json:"priority"`
	Reason          string `json:"reason"`
}

// parseSchedulerResponse menerima `{"sessions":[...]}`, `{"data":[...]}`, atau array polos,
// sekaligus mengembalikan metadata engine (`source`, `warnings`, `fallback_reason`).
func parseSchedulerResponse(raw json.RawMessage) ([]slotRekomendasi, aiSchedulerMeta) {
	var wrapper struct {
		Sessions       []aiSlotRaw `json:"sessions"`
		Data           []aiSlotRaw `json:"data"`
		Source         string      `json:"source"`
		Warnings       []string    `json:"warnings"`
		FallbackReason string      `json:"fallback_reason"`
	}
	meta := aiSchedulerMeta{}
	var candidates [][]aiSlotRaw
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		meta.Source = wrapper.Source
		meta.Warnings = wrapper.Warnings
		meta.FallbackReason = wrapper.FallbackReason
		if len(wrapper.Sessions) > 0 {
			candidates = append(candidates, wrapper.Sessions)
		}
		if len(wrapper.Data) > 0 {
			candidates = append(candidates, wrapper.Data)
		}
	}
	var bare []aiSlotRaw
	if err := json.Unmarshal(raw, &bare); err == nil && len(bare) > 0 {
		candidates = append(candidates, bare)
	}
	if len(candidates) == 0 {
		return nil, meta
	}

	rawSlots := candidates[0]
	slots := make([]slotRekomendasi, 0, len(rawSlots))
	for _, r := range rawSlots {
		date := r.RecommendedDate
		if date == "" {
			date = r.SessionDate
		}
		endTime := r.EndTime
		if endTime == "" && r.StartTime != "" && r.Duration > 0 {
			if start := hhmmMinutes(r.StartTime); start >= 0 {
				endTime = minutesToHHMM(start + r.Duration)
			}
		}
		slots = append(slots, slotRekomendasi{
			TaskID:          r.TaskID,
			RecommendedDate: date,
			StartTime:       r.StartTime,
			EndTime:         endTime,
			Duration:        r.Duration,
			TargetProgress:  r.TargetProgress,
			Priority:        r.Priority,
			Reason:          r.Reason,
		})
	}
	return slots, meta
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
		Where("user_id = ? AND ai_generated = ? AND status <> ?", user.ID, true, migrations.SessionSkipped).
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
		Preload("Task").Preload("Task.Subject").
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

	prediction, err := ctrl.panggilAIPredictor(konteks, user, subject, history)
	if err != nil {
		teruskanKegagalanAI(konteks, err)
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
		AISource:          prediction.Source,
		SyllabusReference: prediction.SyllabusReference,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&record).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.Created(konteks, "prediksi materi dibuat", record)
}

type hasilPrediksi struct {
	PredictedMaterial string  `json:"predicted_topic"`
	Confidence        float64 `json:"confidence"`
	Reason            string  `json:"reason"`
	Prerequisites     string  `json:"prerequisites"`
	Preparation       string  `json:"preparation_suggestion"`
	// Metadata dari engine (diteruskan apa adanya).
	Source            string `json:"source,omitempty"`
	SyllabusReference string `json:"syllabus_reference,omitempty"`
}

// panggilAIPredictor memanggil AI Spec 2. Kegagalan dikembalikan sebagai error
// (termasuk status HTTP dari service AI); tidak ada prediksi rule-based lokal.
func (ctrl *AIController) panggilAIPredictor(konteks *gin.Context, user migrations.User, subject migrations.Subject, history []migrations.Task) (hasilPrediksi, error) {
	url := os.Getenv("AI_PREDICTOR_URL")
	if url == "" {
		return hasilPrediksi{}, errors.New("AI_PREDICTOR_URL belum diset")
	}
	payload := gin.H{
		"subject_id":  subject.ID,
		"subject":     subject.Name,
		"difficulty":  subject.Difficulty,
		"class_level": kelasRomawi(user.Grade),
		"curriculum":  kurikulumAI(),
		"history":     history,
	}

	var raw json.RawMessage
	if err := postJSON(konteks.Request.Context(), url, payload, &raw); err != nil {
		return hasilPrediksi{}, err
	}
	hasil, ok := parsePredictorResponse(raw)
	if !ok {
		return hasilPrediksi{}, &aiResponseError{
			StatusCode: http.StatusBadGateway,
			Message:    "AI tidak memberikan prediksi materi yang bisa dipakai",
		}
	}
	return hasil, nil
}

// aiPrediksiRaw adalah bentuk mentah respons AI Predictor. Mendukung penamaan
// kontrak (`confidence_score`, `summary`, `prerequisites` array) sekaligus
// penamaan internal (`confidence`, `reason`, `prerequisites` string).
type aiPrediksiRaw struct {
	PredictedMaterial   string          `json:"predicted_topic"`
	Confidence          float64         `json:"confidence"`
	ConfidenceScore     float64         `json:"confidence_score"`
	Reason              string          `json:"reason"`
	Summary             string          `json:"summary"`
	Prerequisites       json.RawMessage `json:"prerequisites"`
	Preparation         string          `json:"preparation_suggestion"`
	KeyConcepts         []string        `json:"key_concepts"`
	SuggestedActivities []string        `json:"suggested_activities"`
	Source              string          `json:"source"`
	SyllabusReference   string          `json:"syllabus_reference"`
	Data                *aiPrediksiRaw  `json:"data"`
}

// parsePredictorResponse menerima bentuk `{...}` maupun `{"data":{...}}`.
func parsePredictorResponse(raw json.RawMessage) (hasilPrediksi, bool) {
	var parsed aiPrediksiRaw
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return hasilPrediksi{}, false
	}
	if parsed.Data != nil && parsed.PredictedMaterial == "" {
		parsed = *parsed.Data
	}
	if parsed.PredictedMaterial == "" {
		return hasilPrediksi{}, false
	}

	confidence := parsed.Confidence
	if confidence <= 0 {
		confidence = parsed.ConfidenceScore
	}
	// AI boleh mengirim 0..1 (0.88) atau 0..100 (88); kolom DB memakai 0..100.
	if confidence > 0 && confidence <= 1 {
		confidence *= 100
	}

	reason := parsed.Reason
	if reason == "" {
		reason = parsed.Summary
	}
	if reason == "" && len(parsed.KeyConcepts) > 0 {
		reason = "Konsep kunci: " + strings.Join(parsed.KeyConcepts, ", ")
	}

	preparation := parsed.Preparation
	if preparation == "" && len(parsed.SuggestedActivities) > 0 {
		preparation = strings.Join(parsed.SuggestedActivities, "; ")
	}

	return hasilPrediksi{
		PredictedMaterial: parsed.PredictedMaterial,
		Confidence:        confidence,
		Reason:            reason,
		Prerequisites:     joinPrerequisites(parsed.Prerequisites),
		Preparation:       preparation,
		Source:            parsed.Source,
		SyllabusReference: parsed.SyllabusReference,
	}, true
}

// kelasRomawi memetakan tingkat kelas ke jenjang yang dipakai engine silabus.
// Kontrak saat ini 10..13; nilai 7..9 tetap dipetakan untuk data lama.
func kelasRomawi(grade int8) string {
	switch grade {
	case 7:
		return "VII"
	case 8:
		return "VIII"
	case 9:
		return "IX"
	case 10:
		return "X"
	case 11:
		return "XI"
	case 12:
		return "XII"
	case 13:
		return "XIII"
	default:
		return ""
	}
}

// kurikulumAI: acuan kurikulum yang dikirim ke engine (bisa diubah lewat env).
func kurikulumAI() string {
	if k := strings.TrimSpace(os.Getenv("AI_CURRICULUM")); k != "" {
		return k
	}
	return "Kurikulum Merdeka"
}

// joinPrerequisites menerima `["a","b"]` maupun `"a, b"`.
func joinPrerequisites(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return strings.Join(list, ", ")
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single
	}
	return ""
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

	// Best-effort: teruskan ke engine agar bisa dipakai memperbaiki prompt berikutnya.
	teruskanFeedbackAI(prediction)

	response.OK(konteks, "umpan balik disimpan", prediction)
}

// teruskanFeedbackAI mengirim umpan balik user ke service AI (tidak pernah menggagalkan request).
func teruskanFeedbackAI(prediction migrations.MaterialPrediction) {
	url := aiFeedbackURL()
	if url == "" {
		return
	}
	payload := gin.H{
		"data_feedback": []gin.H{{
			"prediction_id":   prediction.ID,
			"subject_id":      prediction.SubjectID,
			"predicted_topic": prediction.PredictedMaterial,
			"confidence":      prediction.Confidence,
			"reason":          prediction.Reason,
			"feedback":        prediction.UserFeedback,
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var out json.RawMessage
	if err := postJSON(ctx, url, payload, &out); err != nil {
		log.Printf("ai: gagal meneruskan feedback prediksi %d: %v", prediction.ID, err)
	}
}

// aiFeedbackURL menurunkan endpoint /feedback dari AI_PREDICTOR_URL (…/predict -> …/feedback).
func aiFeedbackURL() string {
	raw := strings.TrimSpace(os.Getenv("AI_PREDICTOR_URL"))
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.Path = strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), "/predict") + "/feedback"
	u.RawQuery = ""
	return u.String()
}

// aiResponseError: kegagalan yang datang dari service AI dan membawa status HTTP.
// Dipakai agar controller bisa meneruskan status (mis. 429 saat LLM kena rate limit)
// beserta Retry-After ke klien.
type aiResponseError struct {
	StatusCode int
	Message    string
	RetryAfter string
}

func (e *aiResponseError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "service AI membalas status " + strconv.Itoa(e.StatusCode)
}

// teruskanKegagalanAI memetakan kegagalan panggilan AI menjadi respons klien.
// Keputusan soal limit/degradasi ada di service Python:
// 429 & 503 diteruskan apa adanya (termasuk Retry-After) supaya aplikasi bisa
// menyarankan "coba lagi nanti"; kegagalan lain (koneksi, timeout, respons rusak) jadi 502.
func teruskanKegagalanAI(konteks *gin.Context, err error) {
	var aiErr *aiResponseError
	if errors.As(err, &aiErr) {
		if aiErr.StatusCode == http.StatusTooManyRequests || aiErr.StatusCode == http.StatusServiceUnavailable {
			pesan := aiErr.Message
			if pesan == "" {
				pesan = "layanan AI sedang sibuk, silakan coba lagi nanti"
			}
			if aiErr.RetryAfter != "" {
				konteks.Header("Retry-After", aiErr.RetryAfter)
			}
			response.Fail(konteks, aiErr.StatusCode, pesan, nil)
			return
		}
		log.Printf("ai: service membalas status %d: %s", aiErr.StatusCode, aiErr.Error())
		response.Fail(konteks, http.StatusBadGateway, "layanan AI tidak dapat dihubungi, silakan coba lagi", nil)
		return
	}

	log.Printf("ai: panggilan gagal: %v", err)
	response.Fail(konteks, http.StatusBadGateway, "layanan AI tidak dapat dihubungi, silakan coba lagi", nil)
}

// pesanDariBodyAI mengambil pesan manusiawi dari body error service AI.
// Mendukung {"message": ...}, {"detail": ...}, {"error": "..."} atau teks polos.
func pesanDariBodyAI(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		switch {
		case payload.Message != "":
			return payload.Message
		case payload.Detail != "":
			return payload.Detail
		case payload.Error != "":
			return payload.Error
		}
	}
	teks := strings.TrimSpace(string(body))
	if len(teks) > 200 {
		teks = teks[:200]
	}
	return teks
}

// postJSON mengirim request ke service AI. Timeout default 15 detik
// (PRD §Non-Fungsional) dan bisa diubah lewat AI_TIMEOUT_SECONDS.
// Status non-2xx dikembalikan sebagai *aiResponseError agar status asli tidak hilang.
func postJSON(ctx context.Context, url string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, aiTimeout())
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
		isi, _ := io.ReadAll(io.LimitReader(res.Body, 8<<10))
		return &aiResponseError{
			StatusCode: res.StatusCode,
			Message:    pesanDariBodyAI(isi),
			RetryAfter: res.Header.Get("Retry-After"),
		}
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type chatRequest struct {
	Message string                   `json:"message" binding:"required"`
	History []map[string]interface{} `json:"history"`
	Context map[string]interface{}   `json:"context"`
}

type chatResponse struct {
	Reply     string `json:"reply"`
	Action    string `json:"action,omitempty"`
	Model     string `json:"model,omitempty"`
	Source    string `json:"source,omitempty"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

func (ctrl *AIController) Chat(konteks *gin.Context) {
	user := currentUser(konteks)
	var input chatRequest
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	url := os.Getenv("AI_CHAT_URL")
	if url == "" {
		url = "http://127.0.0.1:8000/chat"
	}

	if input.History == nil {
		input.History = make([]map[string]interface{}, 0)
	}

	if input.Context == nil {
		input.Context = make(map[string]interface{})
	}
	if _, ok := input.Context["student_name"]; !ok {
		input.Context["student_name"] = user.Name
	}
	if _, ok := input.Context["grade"]; !ok {
		input.Context["grade"] = user.Grade
	}
	if _, ok := input.Context["school"]; !ok {
		input.Context["school"] = user.School
	}

	if _, ok := input.Context["active_tasks"]; !ok {
		var activeTasks []migrations.Task
		ctrl.DB.WithContext(konteks.Request.Context()).
			Where("user_id = ? AND status != ?", user.ID, migrations.StatusDone).
			Order("priority_score DESC, deadline ASC").
			Limit(5).
			Find(&activeTasks)

		taskList := make([]map[string]interface{}, 0, len(activeTasks))
		for _, t := range activeTasks {
			taskList = append(taskList, map[string]interface{}{
				"id":       t.ID,
				"title":    t.Title,
				"deadline": t.Deadline.Format("2006-01-02 15:04"),
			})
		}
		input.Context["active_tasks"] = taskList
	}

	var raw json.RawMessage
	if err := postJSON(konteks.Request.Context(), url, input, &raw); err != nil {
		teruskanKegagalanAI(konteks, err)
		return
	}

	var res chatResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "jawaban asisten ai", res)
}

func minutesToHHMM(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// aiTimeout: batas tunggu panggilan service AI. Default 15 detik,
// dapat diubah lewat AI_TIMEOUT_SECONDS (mis. "5" untuk PRD yang ketat).
func aiTimeout() time.Duration {
	raw := os.Getenv("AI_TIMEOUT_SECONDS")
	if raw == "" {
		return 15 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 15 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
