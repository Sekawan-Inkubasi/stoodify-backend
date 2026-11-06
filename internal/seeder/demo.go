package seeder

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	hashingPassword "stoodify_backend/internal/hashing_password"
	"stoodify_backend/internal/priority"
	"stoodify_backend/migrations"
)

func SeedDemo(db *gorm.DB) {
	db.Logger = logger.Default.LogMode(logger.Silent)
	start := time.Now()

	demoEmail := "naufal@student.id"

	var existingUser migrations.User
	if err := db.Where("email = ?", demoEmail).First(&existingUser).Error; err == nil && existingUser.ID > 0 {
		deleteDemoUserData(db, existingUser.ID)
		db.Unscoped().Delete(&existingUser)
		log.Printf("data demo lama untuk %s dibersihkan", demoEmail)
	}

	passHash, err := hashingPassword.HashPassword("password123")
	if err != nil {
		log.Fatalf("gagal hash password demo: %v", err)
	}

	user := migrations.User{
		Name:         "Muhammad Naufal Rafa Al As'ad",
		Email:        demoEmail,
		PasswordHash: passHash,
		Role:         migrations.RoleStudent,
		School:       "SMK Telkom Sidoarjo",
		Grade:        12,
		Major:        "Sistem Informasi, Jaringan, Aplikasi",
	}

	if err := db.Create(&user).Error; err != nil {
		log.Fatalf("gagal membuat user demo: %v", err)
	}

	pref := migrations.UserPreference{
		UserID:            user.ID,
		StartTime:         "16:00",
		EndTime:           "21:00",
		SessionDuration:   60,
		BreakDuration:     15,
		MaxSessionsPerDay: 3,
		ActiveDays:        "1,2,3,4,5,6,7",
	}
	if err := db.Create(&pref).Error; err != nil {
		log.Fatalf("gagal membuat preferensi demo: %v", err)
	}

	type subjectDef struct {
		Name       string
		Teacher    string
		Difficulty int
		Category   string
		Color      string
		AccentBg   string
	}

	defs := []subjectDef{
		{"IoT", "Pak Indra", 3, "Kejuruan", "#3B82F6", "#EFF6FF"},
		{"PKn-Sejarah", "Bu Okky", 2, "Umum", "#EF4444", "#FEF2F2"},
		{"Sistem Keamanan Jaringan 2", "Pak David", 5, "Kejuruan", "#8B5CF6", "#F5F3FF"},
		{"Kecerdasan Artifisial 2", "Bu Rizka", 5, "Kejuruan", "#EC4899", "#FDF2F8"},
		{"Kreativitas Inovasi Kewirausahaan", "Pak Arganata", 2, "Umum", "#F59E0B", "#FFFBEB"},
		{"Bahasa Inggris", "Miss Ferina", 3, "Umum", "#10B981", "#ECFDF5"},
		{"Pemrograman 3", "Bu Eliza", 3, "Kejuruan", "#6366F1", "#EEF2FF"},
		{"Bahasa Indonesia", "Bu Nafita", 2, "Umum", "#14B8A6", "#F0FDFA"},
		{"Platform as a Service", "Pak Novra", 3, "Kejuruan", "#06B6D4", "#ECFEFF"},
		{"Bahasa Jawa", "Pak Galih", 3, "Muatan Lokal", "#84CC16", "#F7FEE7"},
		{"Bimbingan Konseling", "Bu Sinta", 2, "Pengembangan Diri", "#A855F7", "#FAF5FF"},
		{"Pendidikan Agama Islam", "Bu Laila", 3, "Umum", "#059669", "#ECFDF5"},
		{"Matematika", "Pak Shandi", 5, "Umum", "#F97316", "#FFF7ED"},
		{"PJOK", "Bu Chintia", 3, "Umum", "#64748B", "#F8FAFC"},
		{"Inkubasi (Project jurusan SIJA)", "Bu Ike", 5, "Kejuruan", "#4F46E5", "#EEF2FF"},
	}

	subjMap := make(map[string]uint)
	for _, d := range defs {
		s := migrations.Subject{
			UserID:     user.ID,
			Name:       d.Name,
			Teacher:    d.Teacher,
			Difficulty: d.Difficulty,
			Category:   d.Category,
			Color:      d.Color,
			AccentBg:   d.AccentBg,
		}
		if err := db.Create(&s).Error; err != nil {
			log.Fatalf("gagal membuat subject demo %s: %v", d.Name, err)
		}
		subjMap[d.Name] = s.ID
	}

	type schedDef struct {
		SubjectName string
		Day         string
		Start       string
		End         string
		Teacher     string
	}

	schedules := []schedDef{
		{"IoT", "Monday", "07:40", "10:50", "Pak Indra"},
		{"PKn-Sejarah", "Monday", "10:50", "13:10", "Bu Okky"},
		{"Sistem Keamanan Jaringan 2", "Monday", "13:10", "15:10", "Pak David"},

		{"Kecerdasan Artifisial 2", "Tuesday", "07:00", "08:20", "Bu Rizka"},
		{"Kreativitas Inovasi Kewirausahaan", "Tuesday", "08:20", "10:10", "Pak Arganata"},
		{"Bahasa Inggris", "Tuesday", "10:10", "13:10", "Miss Ferina"},
		{"Pemrograman 3", "Tuesday", "13:10", "15:10", "Bu Eliza"},

		{"Bahasa Indonesia", "Wednesday", "07:00", "09:00", "Bu Nafita"},
		{"Platform as a Service", "Wednesday", "09:30", "11:30", "Pak Novra"},
		{"Bahasa Jawa", "Wednesday", "12:30", "13:50", "Pak Galih"},
		{"Bimbingan Konseling", "Wednesday", "13:50", "15:10", "Bu Sinta"},

		{"Pendidikan Agama Islam", "Thursday", "07:00", "08:20", "Bu Laila"},
		{"Kecerdasan Artifisial 2", "Thursday", "08:20", "10:10", "Bu Rizka"},
		{"Matematika", "Thursday", "10:10", "13:50", "Pak Shandi"},
		{"PJOK", "Thursday", "13:50", "15:10", "Bu Chintia"},

		{"Inkubasi (Project jurusan SIJA)", "Friday", "07:40", "15:10", "Bu Ike"},
	}

	for _, sc := range schedules {
		subjID, ok := subjMap[sc.SubjectName]
		if !ok {
			continue
		}
		s := migrations.Schedule{
			UserID:    user.ID,
			SubjectID: subjID,
			DayOfWeek: sc.Day,
			StartTime: sc.Start,
			EndTime:   sc.End,
			Teacher:   sc.Teacher,
		}
		if err := db.Create(&s).Error; err != nil {
			log.Fatalf("gagal membuat schedule demo: %v", err)
		}
	}

	now := time.Now()
	refYear := now.Year()
	if refYear < 2026 {
		refYear = 2026
	}

	loc := time.Local

	type taskDef struct {
		SubjectName string
		Title       string
		Material    string
		Deadline    time.Time
		Difficulty  int
		Type        string
		EstDuration int
		Notes       string
		Progress    int
	}

	taskDefs := []taskDef{
		{
			SubjectName: "Sistem Keamanan Jaringan 2",
			Title:       "Praktikum hardening linux sesuai instruksi dari LKPD",
			Material:    "Linux Server Hardening",
			Deadline:    time.Date(refYear, time.November, 9, 23, 59, 0, 0, loc),
			Difficulty:  5,
			Type:        "project",
			EstDuration: 120,
			Notes:       "Membuat dokumentasi juga setelah melakukan praktikum",
			Progress:    0,
		},
		{
			SubjectName: "Pendidikan Agama Islam",
			Title:       "Menghitung warisan harta (3 soal)",
			Material:    "Hukum Waris Islam / Faraidh",
			Deadline:    time.Date(refYear, time.November, 10, 23, 59, 0, 0, loc),
			Difficulty:  3,
			Type:        "assignment",
			EstDuration: 60,
			Notes:       "Dikerjakan di buku tulis",
			Progress:    0,
		},
		{
			SubjectName: "Bahasa Jawa",
			Title:       "Membuat crita cerkak sendiri dengan tema kecerdasan buatan",
			Material:    "Nulis Cerkak Gagrag Anyar",
			Deadline:    time.Date(refYear, time.November, 11, 23, 59, 0, 0, loc),
			Difficulty:  5,
			Type:        "assignment",
			EstDuration: 90,
			Notes:       "Tema kecerdasan buatan, minimal 3 halaman",
			Progress:    0,
		},
		{
			SubjectName: "Bahasa Indonesia",
			Title:       "Membuat kerangka teks biografi sastrawan Indonesia",
			Material:    "Teks Biografi",
			Deadline:    time.Date(refYear, time.November, 11, 23, 59, 0, 0, loc),
			Difficulty:  3,
			Type:        "assignment",
			EstDuration: 60,
			Notes:       "Sesuai sastrawan yang telah dipilihkan. Mengumpulkan di myLMS",
			Progress:    20,
		},
		{
			SubjectName: "Platform as a Service",
			Title:       "Praktikum deploy aplikasi node js dan CRUD di cloudflare worker",
			Material:    "Serverless Cloudflare Workers",
			Deadline:    time.Date(refYear, time.November, 11, 23, 59, 0, 0, loc),
			Difficulty:  5,
			Type:        "project",
			EstDuration: 120,
			Notes:       "Deploy live link worker dan submit github repo",
			Progress:    0,
		},
		{
			SubjectName: "Matematika",
			Title:       "Mengerjakan soal latihan transformasi fungsi 10 soal",
			Material:    "Transformasi Fungsi",
			Deadline:    time.Date(refYear, time.November, 12, 23, 59, 0, 0, loc),
			Difficulty:  3,
			Type:        "assignment",
			EstDuration: 60,
			Notes:       "Mengerjakan di buku tulis matematika",
			Progress:    0,
		},
		{
			SubjectName: "Kecerdasan Artifisial 2",
			Title:       "Mencari, mengolah, dan membuat pohon keputusan manual (algoritma Gini)",
			Material:    "Decision Tree & Gini Index",
			Deadline:    time.Date(refYear, time.November, 12, 23, 59, 0, 0, loc),
			Difficulty:  5,
			Type:        "assignment",
			EstDuration: 90,
			Notes:       "Dikerjakan manual dengan perhitungan di buku tulis",
			Progress:    0,
		},
		{
			SubjectName: "IoT",
			Title:       "Project IoT - Uji Kenaikan Level (Ujian Akhir Kejuruan)",
			Material:    "Smart Agriculture / Industrial IoT",
			Deadline:    time.Date(refYear, time.November, 30, 23, 59, 0, 0, loc),
			Difficulty:  5,
			Type:        "project",
			EstDuration: 240,
			Notes:       "Projek uji kenaikan level (ujian akhir jurusan)",
			Progress:    10,
		},
	}

	budget := pref.SessionDuration * 3
	for _, td := range taskDefs {
		subjID, ok := subjMap[td.SubjectName]
		if !ok {
			continue
		}

		score := priority.Score(priority.Input{
			Deadline:        td.Deadline,
			Difficulty:      td.Difficulty,
			EstDuration:     td.EstDuration,
			Type:            td.Type,
			Progress:        td.Progress,
			MaxDailyMinutes: budget,
			Now:             now,
		})

		status := migrations.StatusPending
		if td.Progress > 0 {
			status = migrations.StatusInProgress
		}

		t := migrations.Task{
			UserID:            user.ID,
			SubjectID:         subjID,
			Title:             td.Title,
			MaterialName:      td.Material,
			Description:       td.Notes,
			Type:              td.Type,
			Deadline:          td.Deadline,
			Difficulty:        td.Difficulty,
			EstimatedDuration: td.EstDuration,
			PriorityScore:     score,
			Status:            status,
			Progress:          td.Progress,
			Notes:             td.Notes,
		}
		if err := db.Create(&t).Error; err != nil {
			log.Fatalf("gagal membuat task demo: %v", err)
		}
	}

	activities := []migrations.Activity{
		{
			UserID:     user.ID,
			Title:      "Ekstrakurikuler Robotik / IoT Club",
			StartTime:  "15:30",
			EndTime:    "17:00",
			Recurrence: "weekly",
		},
		{
			UserID:     user.ID,
			Title:      "Olahraga Sore & Futsal",
			StartTime:  "16:00",
			EndTime:    "17:30",
			Recurrence: "weekly",
		},
	}
	for _, act := range activities {
		if err := db.Create(&act).Error; err != nil {
			log.Fatalf("gagal membuat activity demo: %v", err)
		}
	}

	log.Printf("seeder demo selesai dalam %s", time.Since(start).Round(time.Millisecond))
	fmt.Printf("\n=== AKUN DEMO SIAP DIGUNAKAN ===\n")
	fmt.Printf("Nama      : %s\n", user.Name)
	fmt.Printf("Email     : %s\n", user.Email)
	fmt.Printf("Password  : password123\n")
	fmt.Printf("Sekolah   : %s (%s, Kelas %d)\n", user.School, user.Major, user.Grade)
	fmt.Printf("Waktu AI  : %s - %s (Sesi %d mnt, Max %d sesi/hari)\n",
		pref.StartTime, pref.EndTime, pref.SessionDuration, pref.MaxSessionsPerDay)
	fmt.Printf("Mapel     : %d mata pelajaran terdaftar\n", len(defs))
	fmt.Printf("Jadwal    : %d sesi pelajaran (Senin-Jumat)\n", len(schedules))
	fmt.Printf("Tugas     : %d tugas kejuruan & umum aktif dengan priority score\n", len(taskDefs))
	fmt.Printf("================================\n\n")
}

func deleteDemoUserData(db *gorm.DB, userID uint) {
	statements := []string{
		`DELETE FROM notifications WHERE user_id = ?`,
		`DELETE FROM study_sessions WHERE user_id = ?`,
		`DELETE FROM material_predictions WHERE user_id = ?`,
		`DELETE FROM subtasks WHERE task_id IN (SELECT id FROM tasks WHERE user_id = ?)`,
		`DELETE FROM tasks WHERE user_id = ?`,
		`DELETE FROM schedules WHERE user_id = ?`,
		`DELETE FROM activities WHERE user_id = ?`,
		`DELETE FROM user_preferences WHERE user_id = ?`,
		`DELETE FROM subjects WHERE user_id = ?`,
		`DELETE FROM refresh_tokens WHERE user_id = ?`,
		`DELETE FROM user_topic_progress WHERE user_id = ?`,
	}
	for _, stmt := range statements {
		db.Exec(stmt, userID)
	}
}
