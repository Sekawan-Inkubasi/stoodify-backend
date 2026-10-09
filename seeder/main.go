package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"stoodify_backend/config"
	"stoodify_backend/internal/hashing_password"
	"stoodify_backend/internal/priority"
	"stoodify_backend/migrations"
)

// Seeder Stoodify — mengisi database dengan banyak data sintetis untuk uji
// Mobile Dev / demo. Semua data memakai domain email @stoodify.seed sehingga
// tidak tercampur dengan akun asli dan bisa dihapus bersih dengan -reset.
//
// Cara pakai:
//
//	go run ./seeder -users 50 -tasks 40          # default
//	go run ./seeder -users 200 -tasks 100        # skala super besar
//	go run ./seeder -users 50 -tasks 40 -reset   # hapus data seed lama dulu

const seedDomain = "@stoodify.seed"

var (
	numUsers   = flag.Int("users", 50, "jumlah user siswa")
	numTasks   = flag.Int("tasks", 40, "jumlah tugas per user")
	doReset    = flag.Bool("reset", false, "hapus semua data seed sebelum mengisi")
	noProgress = flag.Bool("quiet", false, "jangan cetak progres per tahap")
)

func main() {
	flag.Parse()
	if *numUsers < 1 || *numTasks < 1 {
		log.Fatal("users dan tasks harus >= 1")
	}

	db := config.KonekKeDB()
	// Default gorm logger mencetak tiap query >200ms; seeder = ribuan INSERT, jadi dibisukan.
	db.Logger = logger.Default.LogMode(logger.Silent)
	start := time.Now()

	if *doReset {
		resetSeedData(db)
		log.Println("data seed lama dihapus")
	}

	now := time.Now()
	rng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(*numUsers)))

	// bcrypt cost 14 lambat: hash sekali, dipakai semua user.
	passHash, err := hashingPassword.HashPassword("rahasia123")
	if err != nil {
		log.Fatalf("gagal hash password: %v", err)
	}

	users := seedUsers(db, rng, passHash)
	log.Printf("user: %d dibuat", len(users))

	var totalSubjects, totalSchedules, totalTasks, totalSubtasks int
	var totalActivities, totalSessions, totalNotifs, totalPreds int

	for i, user := range users {
		subjects := seedSubjects(db, rng, user.ID)
		schedules := seedSchedules(db, rng, user.ID, subjects)
		tasks := seedTasks(db, rng, user.ID, subjects, now)
		subtasks := seedSubtasks(db, rng, tasks)
		activities := seedActivities(db, rng, user.ID)
		sessions := seedSessions(db, rng, user.ID, tasks, now)
		notifs := seedNotifications(db, rng, user.ID, tasks, now)
		preds := seedPredictions(db, rng, user.ID, subjects)

		totalSubjects += len(subjects)
		totalSchedules += len(schedules)
		totalTasks += len(tasks)
		totalSubtasks += len(subtasks)
		totalActivities += len(activities)
		totalSessions += len(sessions)
		totalNotifs += len(notifs)
		totalPreds += len(preds)

		if !*noProgress && (i+1)%10 == 0 {
			log.Printf("progres: %d/%d user...", i+1, len(users))
		}
	}

	log.Printf("selesai dalam %s", time.Since(start).Round(time.Millisecond))
	fmt.Printf("\n=== RINGKASAN SEED ===\n")
	fmt.Printf("users        : %d\n", len(users))
	fmt.Printf("preferences  : %d\n", len(users))
	fmt.Printf("subjects     : %d\n", totalSubjects)
	fmt.Printf("schedules    : %d\n", totalSchedules)
	fmt.Printf("tasks        : %d\n", totalTasks)
	fmt.Printf("subtasks     : %d\n", totalSubtasks)
	fmt.Printf("activities   : %d\n", totalActivities)
	fmt.Printf("study_sessions: %d\n", totalSessions)
	fmt.Printf("notifications: %d\n", totalNotifs)
	fmt.Printf("predictions  : %d\n", totalPreds)
	fmt.Printf("total baris  : %d\n",
		len(users)*2+totalSubjects+totalSchedules+totalTasks+totalSubtasks+
			totalActivities+totalSessions+totalNotifs+totalPreds)
	fmt.Printf("\nAkun uji:\n")
	fmt.Printf("  admin  : admin@stoodify.seed / admin123\n")
	fmt.Printf("  siswa  : user001@stoodify.seed s.d. user%03d@stoodify.seed / rahasia123\n", *numUsers)
}

// resetSeedData menghapus semua baris milik user berdomain @stoodify.seed.
// Urutan anak->induk agar tidak melanggar foreign key.
func resetSeedData(db *gorm.DB) {
	statements := []string{
		`DELETE FROM notifications WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM study_sessions WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM material_predictions WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM subtasks WHERE task_id IN (SELECT id FROM tasks WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `'))`,
		`DELETE FROM tasks WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM schedules WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM activities WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM user_preferences WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM subjects WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM refresh_tokens WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%` + seedDomain + `')`,
		`DELETE FROM users WHERE email LIKE '%` + seedDomain + `'`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			log.Fatalf("reset gagal: %v\n%s", err, stmt)
		}
	}
}

var firstName = []string{"Adi", "Bima", "Citra", "Dimas", "Elsa", "Fajar", "Gita", "Hendra",
	"Intan", "Joko", "Kirana", "Lutfi", "Maya", "Nando", "Olivia", "Putra", "Qori", "Rina",
	"Satria", "Tari", "Umar", "Vina", "Wahyu", "Xena", "Yusuf", "Zahra"}

var lastName = []string{"Pratama", "Wijaya", "Saputra", "Hidayat", "Kurniawan", "Nugroho",
	"Ramadhan", "Santoso", "Utami", "Wulandari", "Yanti", "Susanto", "Halim", "Anggraini",
	"Firmansyah", "Handayani", "Maulana", "Purnama"}

var subjectNames = []string{"Matematika", "Fisika", "Kimia", "Biologi", "Bahasa Indonesia",
	"Bahasa Inggris", "Sejarah", "Geografi", "Ekonomi", "Informatika", "Seni Budaya", "PJOK"}

var colors = []string{"#FF5252", "#FF9800", "#FFC107", "#4CAF50", "#00BCD4", "#2196F3",
	"#3F51B5", "#9C27B0", "#E91E63", "#795548", "#607D8B", "#F44336"}

type titleTpl struct {
	a, b string
}

var taskTemplates = []titleTpl{
	{"PR ", " halaman 1-10"}, {"Latihan Soal ", " bab 3"}, {"Rangkuman ", " bab 4"},
	{"Proyek ", " kelompok"}, {"Ujian Harian ", " bab 2"}, {"Tugas Baca ", " artikel 1"},
	{"Portofolio ", " semester"}, {"Membuat Mind Map ", " bab 5"}, {"Presentasi ", " kelompok"},
	{"Soal Ulangan ", " bab 6"},
}

var activityTemplates = []titleTpl{
	{"Latihan ", " Basket"}, {"Les ", " Bahasa Inggris"}, {"Ekskul ", " Pramuka"}, {"Ibadah ", " Rutin"},
	{"Latihan ", " Futsal"}, {"Ekstra ", " Robotik"}, {"Kursus ", " Menggambar"}, {"Belajar ", " Piano"},
}

var reasonTpl = []string{
	"Deadline tinggal %d hari lagi, tingkat kesulitan tinggi",
	"Skor prioritas %.0f, perlu waktu fokus lebih panjang",
	"Progres baru %d%%, sisa tugas masih banyak",
	"Ujian %s sudah dekat, alokasikan waktu belajar ekstra",
	"Tugas ini termasuk prioritas %.0f dari 100, kerjakan lebih dulu",
}

// buildReason memilih template lalu mengisi hanya placeholder yang cocok,
// supaya tidak muncul "%!(EXTRA ...)" saat Sprintf.
func buildReason(rng *rand.Rand, title string, progress int, score float64) string {
	switch i := rng.IntN(len(reasonTpl)); i {
	case 0:
		return fmt.Sprintf(reasonTpl[0], intRange(rng, 1, 7))
	case 1, 4:
		return fmt.Sprintf(reasonTpl[i], score)
	case 2:
		return fmt.Sprintf(reasonTpl[2], progress)
	default:
		return fmt.Sprintf(reasonTpl[3], title)
	}
}

// pick mengambil elemen acak dari slice.
func pick[T any](rng *rand.Rand, items []T) T {
	return items[rng.IntN(len(items))]
}

func intRange(rng *rand.Rand, min, max int) int {
	if max <= min {
		return min
	}
	return min + rng.IntN(max-min+1)
}

func hhmm(minutes int) string {
	return fmt.Sprintf("%02d:%02d", (minutes/60)%24, minutes%60)
}

// seedUsers membuat admin + guru + user siswa dengan preferensi masing-masing.
func seedUsers(db *gorm.DB, rng *rand.Rand, passHash string) []migrations.User {
	admin := migrations.User{
		Name: "Admin Stoodify", Email: "admin" + seedDomain,
		PasswordHash: passHash, Role: migrations.RoleAdmin,
	}

	var users []migrations.User
	users = append(users, admin)

	for i := 0; i < 3; i++ {
		users = append(users, migrations.User{
			Name:         fmt.Sprintf("Guru %s %s", pick(rng, firstName), pick(rng, lastName)),
			Email:        fmt.Sprintf("guru%03d%s", i+1, seedDomain),
			PasswordHash: passHash, Role: migrations.RoleTeacher,
		})
	}

	for i := 0; i < *numUsers; i++ {
		users = append(users, migrations.User{
			Name:         fmt.Sprintf("%s %s", pick(rng, firstName), pick(rng, lastName)),
			Email:        fmt.Sprintf("user%03d%s", i+1, seedDomain),
			PasswordHash: passHash,
			Role:         migrations.RoleStudent,
		})
	}

	if err := db.CreateInBatches(users, 200).Error; err != nil {
		log.Fatalf("gagal membuat user: %v", err)
	}

	// Preferensi 1:1 dengan user.
	prefs := make([]migrations.UserPreference, 0, len(users))
	for _, u := range users {
		start := intRange(rng, 14*60, 17*60)
		prefs = append(prefs, migrations.UserPreference{
			UserID:          u.ID,
			StartTime:       hhmm(start),
			EndTime:         hhmm(start + intRange(rng, 3*60, 6*60)),
			SessionDuration: intRange(rng, 30, 120),
			ActiveDays:      "1,2,3,4,5,6",
		})
	}
	if err := db.CreateInBatches(prefs, 200).Error; err != nil {
		log.Fatalf("gagal membuat preferensi: %v", err)
	}
	return users
}

func seedSubjects(db *gorm.DB, rng *rand.Rand, userID uint) []migrations.Subject {
	count := intRange(rng, 4, 8)
	perm := rng.Perm(len(subjectNames))
	subjects := make([]migrations.Subject, 0, count)
	for i := 0; i < count; i++ {
		subjects = append(subjects, migrations.Subject{
			UserID:     userID,
			Name:       subjectNames[perm[i]],
			Color:      colors[rng.IntN(len(colors))],
			Difficulty: intRange(rng, 1, 5),
		})
	}
	if err := db.CreateInBatches(subjects, 100).Error; err != nil {
		log.Fatalf("user %d: gagal membuat subjects: %v", userID, err)
	}
	return subjects
}

// seedSchedules membuat jadwal mingguan tanpa bentrok dalam satu hari.
func seedSchedules(db *gorm.DB, rng *rand.Rand, userID uint, subjects []migrations.Subject) []migrations.Schedule {
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	var schedules []migrations.Schedule

	for _, day := range days[:intRange(rng, 4, 6)] {
		cursor := intRange(rng, 7*60, 8*60)
		for j := 0; j < intRange(rng, 1, 3); j++ {
			dur := intRange(rng, 45, 120)
			if cursor+dur > 16*60 {
				break
			}
			schedules = append(schedules, migrations.Schedule{
				UserID:    userID,
				SubjectID: pick(rng, subjects).ID,
				DayOfWeek: day,
				StartTime: hhmm(cursor),
				EndTime:   hhmm(cursor + dur),
			})
			cursor += dur + intRange(rng, 15, 45)
		}
	}
	if err := db.CreateInBatches(schedules, 200).Error; err != nil {
		log.Fatalf("user %d: gagal membuat schedules: %v", userID, err)
	}
	return schedules
}

// deadlineBucket: past | today | tomorrow | future, dipakai atur status & notif.
func deadlineBucket(rng *rand.Rand) string {
	roll := rng.IntN(100)
	switch {
	case roll < 30:
		return "past"
	case roll < 45:
		return "today"
	case roll < 60:
		return "tomorrow"
	default:
		return "future"
	}
}

func seedTasks(db *gorm.DB, rng *rand.Rand, userID uint, subjects []migrations.Subject, now time.Time) []migrations.Task {
	types := []string{"assignment", "exam", "project", "reading"}
	tasks := make([]migrations.Task, 0, *numTasks)
	subjCount := len(subjects)

	for i := 0; i < *numTasks; i++ {
		subject := subjects[rng.IntN(subjCount)]
		tpl := pick(rng, taskTemplates)

		var deadline time.Time
		switch deadlineBucket(rng) {
		case "past":
			deadline = now.AddDate(0, 0, -intRange(rng, 1, 30))
		case "today":
			deadline = time.Date(now.Year(), now.Month(), now.Day(), intRange(rng, 8, 23), intRange(rng, 0, 59), 0, 0, now.Location())
		case "tomorrow":
			deadline = now.AddDate(0, 0, 1)
			deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), intRange(rng, 8, 23), intRange(rng, 0, 59), 0, 0, now.Location())
		default:
			deadline = now.AddDate(0, 0, intRange(rng, 2, 14))
		}

		taskType := pick(rng, types)
		difficulty := intRange(rng, 1, 5)
		duration := intRange(rng, 15, 240)
		progress := 0
		status := migrations.StatusPending

		nowOnly := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		deadlineOnly := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, deadline.Location())
		roll := rng.IntN(100)
		switch {
		case deadlineOnly.Before(nowOnly): // past
			if roll < 35 {
				status, progress = migrations.StatusDone, 100
			} else {
				status = migrations.StatusLate
			}
		case deadlineOnly.Equal(nowOnly): // today
			switch {
			case roll < 25:
				status, progress = migrations.StatusDone, 100
			case roll < 65:
				status, progress = migrations.StatusInProgress, intRange(rng, 20, 90)
			}
		case deadlineOnly.Equal(nowOnly.AddDate(0, 0, 1)): // tomorrow
			if roll < 15 {
				status, progress = migrations.StatusDone, 100
			} else if roll < 45 {
				status, progress = migrations.StatusInProgress, intRange(rng, 10, 70)
			}
		default: // future
			if roll < 10 {
				status, progress = migrations.StatusInProgress, intRange(rng, 5, 40)
			}
		}

		task := migrations.Task{
			UserID:            userID,
			SubjectID:         subject.ID,
			Title:             tpl.a + subject.Name + tpl.b,
			Description:       "Dikerjakan sesuai instruksi guru.",
			Type:              taskType,
			Deadline:          deadline,
			Difficulty:        difficulty,
			EstimatedDuration: duration,
			Status:            status,
			Progress:          progress,
			IsSnoozed:         status == migrations.StatusPending && roll < 10,
			Notes:             "",
		}
		task.PriorityScore = priority.Score(priority.Input{
			Deadline:        deadline,
			Difficulty:      difficulty,
			EstDuration:     duration,
			Type:            taskType,
			Progress:        progress,
			MaxDailyMinutes: 120,
			Now:             now,
		})
		tasks = append(tasks, task)
	}

	if err := db.CreateInBatches(tasks, 200).Error; err != nil {
		log.Fatalf("user %d: gagal membuat tasks: %v", userID, err)
	}
	return tasks
}

func seedSubtasks(db *gorm.DB, rng *rand.Rand, tasks []migrations.Task) []migrations.Subtask {
	var subtasks []migrations.Subtask
	for _, task := range tasks {
		n := intRange(rng, 0, 5)
		for j := 0; j < n; j++ {
			subtasks = append(subtasks, migrations.Subtask{
				TaskID:      task.ID,
				Title:       fmt.Sprintf("Bagian %d dari %d — %s", j+1, n, task.Title),
				IsCompleted: task.Status == migrations.StatusDone || (rng.IntN(100) < 40 && task.Progress > 0),
			})
		}
	}
	if err := db.CreateInBatches(subtasks, 300).Error; err != nil {
		log.Fatalf("gagal membuat subtasks: %v", err)
	}
	return subtasks
}

func seedActivities(db *gorm.DB, rng *rand.Rand, userID uint) []migrations.Activity {
	recurrences := []string{"daily", "weekly", "monthly", "none"}
	var activities []migrations.Activity
	for i := 0; i < intRange(rng, 5, 8); i++ {
		tpl := pick(rng, activityTemplates)
		start := intRange(rng, 15*60, 19*60)
		dur := intRange(rng, 45, 120)
		activities = append(activities, migrations.Activity{
			UserID:     userID,
			Title:      tpl.a + tpl.b,
			StartTime:  hhmm(start),
			EndTime:    hhmm(start + dur),
			Recurrence: pick(rng, recurrences),
		})
	}
	if err := db.CreateInBatches(activities, 200).Error; err != nil {
		log.Fatalf("user %d: gagal membuat activities: %v", userID, err)
	}
	return activities
}

// seedSessions: sesi planned untuk tugas aktif, sesi completed/skipped historis.
func seedSessions(db *gorm.DB, rng *rand.Rand, userID uint, tasks []migrations.Task, now time.Time) []migrations.StudySession {
	var sessions []migrations.StudySession
	startSlots := []int{15 * 60, 16 * 60, 17 * 60, 18 * 60, 19 * 60, 20 * 60}

	for _, task := range tasks {
		bucket := "past"
		nowOnly := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		deadlineOnly := time.Date(task.Deadline.Year(), task.Deadline.Month(), task.Deadline.Day(), 0, 0, 0, 0, task.Deadline.Location())
		switch {
		case deadlineOnly.Before(nowOnly):
			bucket = "past"
		case deadlineOnly.Equal(nowOnly), deadlineOnly.Equal(nowOnly.AddDate(0, 0, 1)):
			bucket = "near"
		default:
			bucket = "future"
		}

		// Tugas tidak selesai: tawarkan sesi belajar.
		if task.Status != migrations.StatusDone && bucket != "past" {
			date := task.Deadline
			if date.Before(now) {
				date = now.AddDate(0, 0, 1)
			}
			for s := 0; s < intRange(rng, 1, 2); s++ {
				start := pick(rng, startSlots)
				dur := task.EstimatedDuration
				if dur > 120 {
					dur = 120
				}
				remaining := 100 - task.Progress
				target := 30
				if remaining < target {
					target = remaining
				}
				sessions = append(sessions, migrations.StudySession{
					UserID:         userID,
					TaskID:         task.ID,
					SessionDate:    date,
					StartTime:      hhmm(start),
					Duration:       dur,
					TargetProgress: target,
					Status:         migrations.SessionPlanned,
					AIGenerated:    true,
					Reason:         buildReason(rng, task.Title, task.Progress, task.PriorityScore),
				})
			}
			continue
		}

		// Tugas selesai: sesi historis completed; tugas telat: sesi skipped.
		if task.Status == migrations.StatusDone && rng.IntN(100) < 70 {
			date := task.Deadline.AddDate(0, 0, -intRange(rng, 1, 7))
			sessions = append(sessions, migrations.StudySession{
				UserID:         userID,
				TaskID:         task.ID,
				SessionDate:    date,
				StartTime:      hhmm(pick(rng, startSlots)),
				Duration:       intRange(rng, 30, 90),
				TargetProgress: task.Progress,
				Status:         migrations.SessionCompleted,
			})
		} else if task.Status == migrations.StatusLate && rng.IntN(100) < 40 {
			sessions = append(sessions, migrations.StudySession{
				UserID:      userID,
				TaskID:      task.ID,
				SessionDate: task.Deadline,
				StartTime:   hhmm(pick(rng, startSlots)),
				Duration:    intRange(rng, 30, 90),
				Status:      migrations.SessionSkipped,
			})
		}
	}
	if err := db.CreateInBatches(sessions, 300).Error; err != nil {
		log.Fatalf("user %d: gagal membuat sessions: %v", userID, err)
	}
	return sessions
}

func seedNotifications(db *gorm.DB, rng *rand.Rand, userID uint, tasks []migrations.Task, now time.Time) []migrations.Notification {
	var notifs []migrations.Notification
	nowOnly := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, task := range tasks {
		deadlineOnly := time.Date(task.Deadline.Year(), task.Deadline.Month(), task.Deadline.Day(), 0, 0, 0, 0, task.Deadline.Location())
		taskID := task.ID
		switch {
		case task.Status == migrations.StatusLate:
			if rng.IntN(100) < 60 {
				notifs = append(notifs, migrations.Notification{
					UserID: userID, TaskID: &taskID, Type: migrations.NotifLate,
					Title: "Tugas Terlambat",
					Message: fmt.Sprintf("Tugas \"%s\" sudah melewati deadline %s.",
						task.Title, task.Deadline.Format("02 Jan 2006 15:04")),
					ScheduledAt: task.Deadline.Add(time.Hour), IsRead: rng.IntN(100) < 40,
				})
			}
		case deadlineOnly.Equal(nowOnly) && task.Status != migrations.StatusDone:
			notifs = append(notifs, migrations.Notification{
				UserID: userID, TaskID: &taskID, Type: migrations.NotifReminder,
				Title: "Deadline hari ini",
				Message: fmt.Sprintf("Tugas \"%s\" harus selesai hari ini %s.",
					task.Title, task.Deadline.Format("15:04")),
				ScheduledAt: nowOnly.Add(8 * time.Hour), IsRead: rng.IntN(100) < 30,
			})
		case deadlineOnly.Equal(nowOnly.AddDate(0, 0, 1)) && task.Status != migrations.StatusDone:
			notifs = append(notifs, migrations.Notification{
				UserID: userID, TaskID: &taskID, Type: migrations.NotifReminder,
				Title: "Deadline besok",
				Message: fmt.Sprintf("Tugas \"%s\" jatuh tempo besok %s.",
					task.Title, task.Deadline.Format("02 Jan 2006 15:04")),
				ScheduledAt: nowOnly.Add(9 * time.Hour), IsRead: rng.IntN(100) < 30,
			})
		default:
			if task.Status != migrations.StatusDone && rng.IntN(100) < 15 {
				notifs = append(notifs, migrations.Notification{
					UserID: userID, TaskID: &taskID, Type: migrations.NotifWarning,
					Title: "Jangan lupa kerjakan",
					Message: fmt.Sprintf("Sisihkan waktu untuk \"%s\" sebelum deadline %s.",
						task.Title, task.Deadline.Format("02 Jan 2006")),
					ScheduledAt: task.Deadline.AddDate(0, 0, -2), IsRead: rng.IntN(100) < 50,
				})
			}
		}
	}
	if err := db.CreateInBatches(notifs, 300).Error; err != nil {
		log.Fatalf("user %d: gagal membuat notifications: %v", userID, err)
	}
	return notifs
}

func seedPredictions(db *gorm.DB, rng *rand.Rand, userID uint, subjects []migrations.Subject) []migrations.MaterialPrediction {
	nextTopics := []string{"Integral Dasar", "Turunan Fungsi", "Reaksi Redoks", "Genetika", "Proklamasi",
		"Persamaan Kuadrat", "Teks Eksposisi", "Termodinamika", "Vektor", "Stoikiometri"}
	var preds []migrations.MaterialPrediction
	for _, subject := range subjects {
		for p := 0; p < intRange(rng, 1, 3); p++ {
			confidence := float64(intRange(rng, 30, 92))
			preds = append(preds, migrations.MaterialPrediction{
				UserID:            userID,
				SubjectID:         subject.ID,
				PredictedMaterial: pick(rng, nextTopics) + " (" + subject.Name + ")",
				Confidence:        confidence,
				Reason:            fmt.Sprintf("Berdasarkan pola %d tugas terakhir pada %s", intRange(rng, 3, 12), subject.Name),
				Prerequisites:     subject.Name + " bab sebelumnya",
				Preparation:       "Ulangi catatan " + subject.Name + " sebelum kelas berikutnya",
			})
		}
	}
	if err := db.CreateInBatches(preds, 300).Error; err != nil {
		log.Fatalf("user %d: gagal membuat predictions: %v", userID, err)
	}
	return preds
}
