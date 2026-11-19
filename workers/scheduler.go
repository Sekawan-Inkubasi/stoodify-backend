package workers

import (
	"log"
	"time"

	"gorm.io/gorm"

	"stoodify_backend/migrations"
)

// JalankanScheduler: ticker 1 menit, melakukan:
//  1. tandai tugas lewat deadline jadi "late"
//  2. kirim notifikasi H-1 dan H-0
//  3. rollover harian (00:05): minta planner menyusun jadwal untuk hari yang baru mulai
//
// ponytail: ticker 1 menit, bukan robfig/cron — cek berbasis waktu absolut
// tidak butuh ekspresi cron. Ganti ke robfig/cron kalau perlu jadwal kalender.
func JalankanScheduler(db *gorm.DB, berhenti <-chan struct{}, picuRegenerasi func(userID uint)) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	jalankanSekali(db)
	terakhirRollover := ""

	for {
		select {
		case <-berhenti:
			log.Println("scheduler dihentikan")
			return
		case sekarang := <-ticker.C:
			jalankanSekali(db)
			if picuRegenerasi != nil {
				terakhirRollover = rolloverHarian(db, sekarang, terakhirRollover, picuRegenerasi)
			}
		}
	}
}

// rolloverHarian: sekali sehari setelah 00:05, picu regenerasi untuk user yang aktif
// 30 hari terakhir supaya jadwal hari ini sudah siap tanpa menunggu app dibuka.
func rolloverHarian(db *gorm.DB, sekarang time.Time, terakhir string, picu func(userID uint)) string {
	tanggal := sekarang.Format("2006-01-02")
	if sekarang.Hour() != 0 || sekarang.Minute() < 5 || terakhir == tanggal {
		return terakhir
	}

	var users []migrations.User
	batas := sekarang.AddDate(0, 0, -30)
	if err := db.Model(&migrations.User{}).
		Where("updated_at >= ?", batas).
		Order("updated_at DESC").Limit(500).Find(&users).Error; err != nil {
		log.Printf("rollover: gagal mengambil daftar user: %v", err)
		return terakhir
	}
	for _, u := range users {
		picu(u.ID)
	}
	log.Printf("rollover: %d user dijadwalkan ulang untuk %s", len(users), tanggal)
	return tanggal
}

func jalankanSekali(db *gorm.DB) {
	if n, err := TandaiTerlambat(db); err != nil {
		log.Printf("scheduler: gagal tandai terlambat: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: %d tugas ditandai terlambat", n)
	}

	if n, err := BuatNotifikasiDeadline(db); err != nil {
		log.Printf("scheduler: gagal buat notifikasi: %v", err)
	} else if n > 0 {
		log.Printf("scheduler: %d notifikasi deadline dibuat", n)
	}
}

// TandaiTerlambat: pending/in_progress yang lewat deadline -> late.
func TandaiTerlambat(db *gorm.DB) (int64, error) {
	result := db.Model(&migrations.Task{}).
		Where("deadline < ? AND status IN ?", time.Now(),
			[]string{migrations.StatusPending, migrations.StatusInProgress}).
		Update("status", migrations.StatusLate)
	return result.RowsAffected, result.Error
}

// BuatNotifikasiDeadline membuat notifikasi H-1 dan H-0, hanya sekali per tugas.
func BuatNotifikasiDeadline(db *gorm.DB) (int64, error) {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var tasks []migrations.Task
	err := db.Where("status IN ?", []string{migrations.StatusPending, migrations.StatusInProgress}).
		Where("deadline >= ? AND deadline < ?", startOfToday, startOfToday.AddDate(0, 0, 2)).
		Find(&tasks).Error
	if err != nil {
		return 0, err
	}

	var dibuat int64
	for i := range tasks {
		task := tasks[i]
		deadlineDay := time.Date(task.Deadline.Year(), task.Deadline.Month(), task.Deadline.Day(), 0, 0, 0, 0, now.Location())
		selisihHari := int(deadlineDay.Sub(startOfToday).Hours() / 24)

		var jenis, judul string
		switch selisihHari {
		case 0:
			jenis, judul = migrations.NotifReminder, "Deadline hari ini"
		case 1:
			jenis, judul = migrations.NotifReminder, "Deadline besok"
		default:
			continue
		}

		var jumlah int64
		db.Model(&migrations.Notification{}).
			Where("task_id = ? AND type = ? AND scheduled_at >= ?", task.ID, jenis, startOfToday).
			Count(&jumlah)
		if jumlah > 0 {
			continue // sudah pernah dikirim hari ini
		}

		pesan := "Tugas \"" + task.Title + "\" harus selesai " + task.Deadline.Format("02 Jan 2006 15:04")
		taskID := task.ID
		notif := migrations.Notification{
			UserID:      task.UserID,
			TaskID:      &taskID,
			Title:       judul,
			Message:     pesan,
			Type:        jenis,
			ScheduledAt: now,
		}
		if err := db.Create(&notif).Error; err != nil {
			return dibuat, err
		}
		dibuat++
	}
	return dibuat, nil
}
