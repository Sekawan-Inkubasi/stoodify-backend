package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"stoodify_backend/config"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

func TestParseSchedulerResponseBentukRespons(t *testing.T) {
	slot := `{"task_id":7,"start_time":"16:30","duration":60,"target_progress":30,"priority":"high","reason":"deadline dekat"}`

	cases := []struct {
		name     string
		raw      string
		wantLen  int
		wantDate string
		wantEnd  string
	}{
		{name: "sessions internal", raw: `{"sessions":[` + slot + `]}`, wantLen: 1, wantEnd: "17:30"},
		{name: "kontrak session_date", raw: `{"data":[{"task_id":7,"session_date":"2026-11-14","start_time":"16:30","end_time":"17:30","duration":60}]}`, wantLen: 1, wantDate: "2026-11-14", wantEnd: "17:30"},
		{name: "array polos", raw: `[` + slot + `]`, wantLen: 1, wantEnd: "17:30"},
		{name: "sessions menang atas data", raw: `{"sessions":[` + slot + `],"data":[` + slot + `,` + slot + `]}`, wantLen: 1, wantEnd: "17:30"},
		{name: "tidak ada slot", raw: `{"sessions":[]}`, wantLen: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slots, _ := parseSchedulerResponse(json.RawMessage(tc.raw))
			if len(slots) != tc.wantLen {
				t.Fatalf("jumlah slot = %d, mau %d", len(slots), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if slots[0].TaskID != 7 {
				t.Errorf("task_id = %d, mau 7", slots[0].TaskID)
			}
			if slots[0].RecommendedDate != tc.wantDate {
				t.Errorf("tanggal = %q, mau %q", slots[0].RecommendedDate, tc.wantDate)
			}
			if slots[0].EndTime != tc.wantEnd {
				t.Errorf("end_time = %q, mau %q", slots[0].EndTime, tc.wantEnd)
			}
		})
	}
}

func TestParsePredictorResponseBentukRespons(t *testing.T) {
	kontrak := `{"predicted_topic":"Integral Tentu","confidence_score":0.88,"summary":"ringkasan","prerequisites":["Turunan Dasar","Integral Substitusi"],"key_concepts":["Teorema Dasar"],"suggested_activities":["Latihan A","Latihan B"]}`
	internal := `{"predicted_topic":"Limit Fungsi","confidence":72,"reason":"karena X","prerequisites":"Aljabar, Fungsi","preparation_suggestion":"Baca bab 3"}`

	t.Run("gaya kontrak dinormalisasi", func(t *testing.T) {
		hasil, ok := parsePredictorResponse(json.RawMessage(kontrak))
		if !ok {
			t.Fatal("respons kontrak ditolak")
		}
		if hasil.PredictedMaterial != "Integral Tentu" {
			t.Errorf("predicted = %q", hasil.PredictedMaterial)
		}
		if hasil.Confidence != 88 {
			t.Errorf("confidence = %v, mau 88 (0.88 dikali 100)", hasil.Confidence)
		}
		if hasil.Reason != "ringkasan" {
			t.Errorf("reason = %q, mau dari summary", hasil.Reason)
		}
		if hasil.Prerequisites != "Turunan Dasar, Integral Substitusi" {
			t.Errorf("prerequisites = %q", hasil.Prerequisites)
		}
		if hasil.Preparation != "Latihan A; Latihan B" {
			t.Errorf("preparation = %q, mau dari suggested_activities", hasil.Preparation)
		}
	})

	t.Run("dibungkus data", func(t *testing.T) {
		hasil, ok := parsePredictorResponse(json.RawMessage(`{"data":` + internal + `}`))
		if !ok {
			t.Fatal("respons ber-wrapper data ditolak")
		}
		if hasil.Confidence != 72 || hasil.Reason != "karena X" || hasil.Prerequisites != "Aljabar, Fungsi" {
			t.Errorf("normalisasi salah: %+v", hasil)
		}
	})

	t.Run("tanpa topik dianggap gagal", func(t *testing.T) {
		if _, ok := parsePredictorResponse(json.RawMessage(`{"confidence":90}`)); ok {
			t.Error("respons tanpa predicted_topic seharusnya dianggap gagal (tidak ada prediksi lokal)")
		}
	})
}

func TestAiTimeoutDefault(t *testing.T) {
	t.Setenv("AI_TIMEOUT_SECONDS", "")
	if got := aiTimeout().Seconds(); got != 15 {
		t.Errorf("timeout default = %v detik, mau 15", got)
	}
	t.Setenv("AI_TIMEOUT_SECONDS", "5")
	if got := aiTimeout().Seconds(); got != 5 {
		t.Errorf("timeout env = %v detik, mau 5", got)
	}
	t.Setenv("AI_TIMEOUT_SECONDS", "bukan-angka")
	if got := aiTimeout().Seconds(); got != 15 {
		t.Errorf("timeout invalid = %v detik, mau 15", got)
	}
}

// ==================== Penanganan kegagalan service AI ====================

func TestTeruskanKegagalanAI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantRetry  string
		wantPesan  string
	}{
		{
			name:       "429 dari Python diteruskan apa adanya",
			err:        &aiResponseError{StatusCode: http.StatusTooManyRequests, Message: "kuota LLM habis", RetryAfter: "30"},
			wantStatus: http.StatusTooManyRequests,
			wantRetry:  "30",
			wantPesan:  "kuota LLM habis",
		},
		{
			name:       "503 dari Python diteruskan",
			err:        &aiResponseError{StatusCode: http.StatusServiceUnavailable, Message: "model sedang dimuat"},
			wantStatus: http.StatusServiceUnavailable,
			wantPesan:  "model sedang dimuat",
		},
		{
			name:       "429 tanpa pesan pakai pesan default",
			err:        &aiResponseError{StatusCode: http.StatusTooManyRequests},
			wantStatus: http.StatusTooManyRequests,
			wantPesan:  "layanan AI sedang sibuk",
		},
		{
			name:       "status lain jadi 502 tanpa membocorkan detail",
			err:        &aiResponseError{StatusCode: http.StatusInternalServerError, Message: "traceback internal python"},
			wantStatus: http.StatusBadGateway,
			wantPesan:  "layanan AI tidak dapat dihubungi",
		},
		{
			name:       "koneksi mati jadi 502",
			err:        errors.New("dial tcp 127.0.0.1:8000: connect: connection refused"),
			wantStatus: http.StatusBadGateway,
			wantPesan:  "layanan AI tidak dapat dihubungi",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			konteks, _ := gin.CreateTestContext(w)
			konteks.Request = httptest.NewRequest(http.MethodPost, "/api/ai", nil)

			teruskanKegagalanAI(konteks, tc.err)

			assert.Equal(t, tc.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tc.wantPesan)
			assert.Equal(t, tc.wantRetry, w.Header().Get("Retry-After"))
			if tc.wantStatus == http.StatusBadGateway {
				assert.NotContains(t, w.Body.String(), "traceback")
			}
		})
	}
}

func TestPostJSONMenjagaStatusDanPesanAI(t *testing.T) {
	// 429 dengan Retry-After + body pesan Python
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "45")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"Kuota LLM harian habis"}`))
	}))
	defer srv.Close()

	var out json.RawMessage
	err := postJSON(t.Context(), srv.URL, map[string]string{"x": "y"}, &out)
	require.Error(t, err)

	var aiErr *aiResponseError
	require.True(t, errors.As(err, &aiErr), "error harus bertipe *aiResponseError")
	assert.Equal(t, http.StatusTooManyRequests, aiErr.StatusCode)
	assert.Equal(t, "Kuota LLM harian habis", aiErr.Message)
	assert.Equal(t, "45", aiErr.RetryAfter)

	// body gaya FastAPI (`detail`) juga harus terbaca
	srvDetail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"detail":"model belum siap"}`))
	}))
	defer srvDetail.Close()

	err = postJSON(t.Context(), srvDetail.URL, map[string]string{}, &out)
	require.Error(t, err)
	require.True(t, errors.As(err, &aiErr))
	assert.Equal(t, "model belum siap", aiErr.Message)
}

// ==================== Handler: AI mati / limit / sukses ====================

func setupAITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&migrations.User{},
		&migrations.UserPreference{},
		&migrations.Subject{},
		&migrations.Task{},
		&migrations.Schedule{},
		&migrations.Activity{},
		&migrations.StudySession{},
		&migrations.MaterialPrediction{},
		&migrations.ScheduleSnapshot{},
		&migrations.RefreshToken{},
		&migrations.PasswordResetToken{},
		&migrations.EmailVerificationToken{},
	))
	config.DB = db

	// Reset batas "Susun ulang" manual antar test (state-nya global per proses).
	susunUlangManual.mu.Lock()
	susunUlangManual.terakhir = make(map[uint]time.Time)
	susunUlangManual.mu.Unlock()

	return db
}

func setupAITestRouter(t *testing.T, db *gorm.DB) (*gin.Engine, migrations.User, migrations.Task, migrations.Subject) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	user := migrations.User{Name: "Siswa AI", Email: "siswa-ai@example.com", PasswordHash: "x", Role: migrations.RoleStudent}
	require.NoError(t, db.Create(&user).Error)

	task := migrations.Task{
		UserID:            user.ID,
		Title:             "Latihan Integral",
		Deadline:          time.Now().Add(48 * time.Hour),
		Difficulty:        3,
		EstimatedDuration: 60,
		PriorityScore:     80,
		Status:            migrations.StatusPending,
	}
	require.NoError(t, db.Create(&task).Error)

	subject := migrations.Subject{UserID: user.ID, Name: "Matematika", Difficulty: 3}
	require.NoError(t, db.Create(&subject).Error)

	ctrl := NewAIController(db)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middlewares.ContextUserKey, user)
		c.Next()
	})
	r.POST("/api/ai/schedule-recommendations/generate", ctrl.GenerateSchedule)
	r.POST("/api/ai/material-predictions/generate", ctrl.GeneratePrediction)

	return r, user, task, subject
}

func TestGenerateScheduleTanpaKonfigurasiAI(t *testing.T) {
	db := setupAITestDB(t)
	r, user, _, _ := setupAITestRouter(t, db)
	t.Setenv("AI_SCHEDULER_URL", "")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/schedule-recommendations/generate", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "layanan AI tidak dapat dihubungi")

	var jumlah int64
	require.NoError(t, db.Model(&migrations.StudySession{}).Where("user_id = ?", user.ID).Count(&jumlah).Error)
	assert.Zero(t, jumlah, "tidak boleh ada sesi tersimpan saat AI gagal")
}

func TestGenerateScheduleRateLimitDiteruskan(t *testing.T) {
	db := setupAITestDB(t)
	r, user, _, _ := setupAITestRouter(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"Kuota LLM harian habis"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SCHEDULER_URL", srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/schedule-recommendations/generate", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "30", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "Kuota LLM harian habis")

	var jumlah int64
	require.NoError(t, db.Model(&migrations.StudySession{}).Where("user_id = ?", user.ID).Count(&jumlah).Error)
	assert.Zero(t, jumlah, "rate limit tidak boleh menyimpan sesi parsial")
}

func TestGenerateScheduleSuksesDariAI(t *testing.T) {
	db := setupAITestDB(t)
	r, user, task, _ := setupAITestRouter(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[{"task_id":` + strconv.FormatUint(uint64(task.ID), 10) +
			`,"session_date":"2026-11-16","start_time":"16:00","duration":60,"target_progress":30,"priority":"high","reason":"deadline dekat"}]}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SCHEDULER_URL", srv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/schedule-recommendations/generate", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var sesi migrations.StudySession
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&sesi).Error)
	assert.Equal(t, task.ID, sesi.TaskID)
	assert.Equal(t, "16:00", sesi.StartTime)
	assert.True(t, sesi.AIGenerated)
}

func TestGeneratePredictionRateLimitDiteruskan(t *testing.T) {
	db := setupAITestDB(t)
	r, user, _, subject := setupAITestRouter(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"limit harian LLM tercapai"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_PREDICTOR_URL", srv.URL)

	body := `{"subject_id":` + strconv.FormatUint(uint64(subject.ID), 10) + `}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/material-predictions/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "20", w.Header().Get("Retry-After"))
	assert.Contains(t, w.Body.String(), "limit harian LLM tercapai")

	var jumlah int64
	require.NoError(t, db.Model(&migrations.MaterialPrediction{}).Where("user_id = ?", user.ID).Count(&jumlah).Error)
	assert.Zero(t, jumlah, "rate limit tidak boleh menyimpan prediksi parsial")
}

// ==================== Auto-schedule: metadata, locked, limit, undo ====================

func TestParseSchedulerResponseMeta(t *testing.T) {
	raw := `{"sessions":[{"task_id":7,"start_time":"16:00","duration":60}],"source":"fallback","warnings":["tugas X tanpa deadline"],"fallback_reason":"LLM 503"}`
	slots, meta := parseSchedulerResponse(json.RawMessage(raw))

	require.Len(t, slots, 1)
	assert.Equal(t, "fallback", meta.Source)
	assert.Equal(t, []string{"tugas X tanpa deadline"}, meta.Warnings)
	assert.Equal(t, "LLM 503", meta.FallbackReason)
}

func TestSusunSesiMenghormatiSesiTerkunci(t *testing.T) {
	db := setupAITestDB(t)
	gin.SetMode(gin.TestMode)

	user := migrations.User{Name: "Siswa Pin", Email: "pin@example.com", PasswordHash: "x"}
	require.NoError(t, db.Create(&user).Error)
	task := migrations.Task{UserID: user.ID, Title: "Tugas", Deadline: time.Now().Add(48 * time.Hour),
		Difficulty: 3, EstimatedDuration: 60, Status: migrations.StatusPending}
	require.NoError(t, db.Create(&task).Error)

	// Sesi milik user (pinned) 16:00-17:00 hari ini.
	pinned := migrations.StudySession{
		UserID: user.ID, TaskID: task.ID, SessionDate: time.Now(),
		StartTime: "16:00", Duration: 60, Status: migrations.SessionPlanned, AIGenerated: false,
	}
	require.NoError(t, db.Create(&pinned).Error)

	ctrl := NewAIController(db)
	k := konteksJadwal{
		User:     user,
		Tanggal:  time.Now(),
		Pref:     migrations.UserPreference{SessionDuration: 60},
		Existing: []migrations.StudySession{pinned},
	}
	slots := []slotRekomendasi{
		{TaskID: task.ID, StartTime: "16:30", Duration: 60}, // bentrok dengan pinned
		{TaskID: task.ID, StartTime: "19:00", Duration: 60}, // aman
	}

	hasil := ctrl.susunSesi(k, slots, aiSchedulerMeta{Source: "ai"})
	require.Len(t, hasil, 1, "slot yang bentrok dengan sesi pinned harus dibuang")
	assert.Equal(t, "19:00", hasil[0].StartTime)
	assert.Equal(t, "ai", hasil[0].AISource)
}

func TestGenerateScheduleRateLimitManual(t *testing.T) {
	db := setupAITestDB(t)
	r, user, task, _ := setupAITestRouter(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sessions":[{"task_id":` + strconv.FormatUint(uint64(task.ID), 10) +
			`,"recommended_date":"` + time.Now().Format("2006-01-02") + `","start_time":"19:00","duration":60}],"source":"ai"}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SCHEDULER_URL", srv.URL)

	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/ai/schedule-recommendations/generate", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	pertama := call()
	assert.Equal(t, http.StatusCreated, pertama.Code)

	kedua := call()
	assert.Equal(t, http.StatusTooManyRequests, kedua.Code)
	assert.Contains(t, kedua.Body.String(), "Coba lagi dalam")
	assert.NotEmpty(t, kedua.Header().Get("Retry-After"))

	// Sesi tidak bertambah karena request kedua ditolak.
	var jumlah int64
	require.NoError(t, db.Model(&migrations.StudySession{}).Where("user_id = ?", user.ID).Count(&jumlah).Error)
	assert.Equal(t, int64(1), jumlah)
}

func TestUndoRegenerasiMengembalikanJadwal(t *testing.T) {
	db := setupAITestDB(t)
	gin.SetMode(gin.TestMode)

	user := migrations.User{Name: "Siswa Undo", Email: "undo@example.com", PasswordHash: "x"}
	require.NoError(t, db.Create(&user).Error)
	task := migrations.Task{UserID: user.ID, Title: "Tugas", Deadline: time.Now().Add(48 * time.Hour),
		Difficulty: 3, EstimatedDuration: 60, Status: migrations.StatusPending}
	require.NoError(t, db.Create(&task).Error)

	// Kondisi sekarang: sesi AI jam 19:00.
	require.NoError(t, db.Create(&migrations.StudySession{
		UserID: user.ID, TaskID: task.ID, SessionDate: time.Now(),
		StartTime: "19:00", Duration: 60, Status: migrations.SessionPlanned, AIGenerated: true,
	}).Error)

	// Snapshot berisi jadwal sebelumnya (jam 16:00).
	lama, err := json.Marshal([]migrations.StudySession{{
		UserID: user.ID, TaskID: task.ID, SessionDate: time.Now(),
		StartTime: "16:00", Duration: 60, Status: migrations.SessionPlanned, AIGenerated: true,
	}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&migrations.ScheduleSnapshot{
		UserID: user.ID, SessionDate: time.Now(), Payload: string(lama),
	}).Error)

	ctrl := NewAIController(db)
	w := httptest.NewRecorder()
	konteks, _ := gin.CreateTestContext(w)
	konteks.Request = httptest.NewRequest(http.MethodPost, "/api/ai/schedule-recommendations/undo", nil)
	konteks.Set(middlewares.ContextUserKey, user)

	ctrl.UndoRegenerasi(konteks)

	assert.Equal(t, http.StatusOK, w.Code)
	var sesi []migrations.StudySession
	require.NoError(t, db.Where("user_id = ?", user.ID).Find(&sesi).Error)
	require.Len(t, sesi, 1)
	assert.Equal(t, "16:00", sesi[0].StartTime, "jadwal harus kembali ke snapshot")
}
