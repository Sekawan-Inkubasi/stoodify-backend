package migrations

import (
	"time"

	"gorm.io/gorm"
)

// ==================== BASE MODEL ====================

type Base struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // Sembunyikan total dari JSON response
}

// ==================== USER & PREFERENCES ====================

type User struct {
	Base
	Name         string          `gorm:"not null" json:"name"`
	Email        string          `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string          `gorm:"column:password_hash;not null" json:"-"` // Jangan pernah kirim ke client
	Role         string          `gorm:"default:student" json:"role"`
	Preferences  *UserPreference `gorm:"foreignKey:UserID" json:"preferences,omitempty"` // Pointer agar null jika kosong
}

type UserPreference struct {
	Base
	UserID          uint   `gorm:"uniqueIndex;not null" json:"userID"`
	StartTime       string `json:"startTime"`       // Format: "08:00"
	EndTime         string `json:"endTime"`         // Format: "20:00"
	SessionDuration int    `json:"sessionDuration"` // Dalam menit
	ActiveDays      string `json:"activeDays"`      // Format: "1,2,3,4,5" (Senin-Jumat)
}

// ==================== SUBJECT & SCHEDULE ====================

type Subject struct {
	Base
	UserID     uint   `gorm:"index;not null" json:"userID"`
	Name       string `gorm:"not null" json:"name"`
	Color      string `json:"color"`
	Difficulty int    `gorm:"check:difficulty >= 1 AND difficulty <= 5" json:"difficulty"`
}

type Schedule struct {
	Base
	UserID    uint   `gorm:"index;not null" json:"userID"`
	SubjectID uint   `gorm:"index" json:"subjectID"`
	DayOfWeek string `json:"dayOfWeek"` // e.g., "Monday"
	StartTime string `json:"startTime"` // e.g., "07:00" (String lebih aman untuk jadwal mingguan tanpa tanggal)
	EndTime   string `json:"endTime"`   // e.g., "08:30"

	User    User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Subject Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

// ==================== TASK & SUBTASK ====================

type Task struct {
	Base
	UserID        uint      `gorm:"index;not null" json:"userID"`
	SubjectID     uint      `gorm:"index" json:"subjectID"`
	Title         string    `gorm:"not null" json:"title"`
	Description   string    `json:"description"`
	Deadline      time.Time `gorm:"index;not null" json:"deadline"`
	Difficulty    int       `gorm:"check:difficulty >= 1 AND difficulty <= 5" json:"difficulty"`
	PriorityScore float64   `json:"priorityScore"`                 // Untuk Priority Engine
	Status        string    `gorm:"default:pending" json:"status"` // pending, in_progress, done, late
	Progress      int       `gorm:"default:0;check:progress >= 0 AND progress <= 100" json:"progress"`
	Subtasks      []Subtask `gorm:"foreignKey:TaskID" json:"subtasks,omitempty"`

	User    User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Subject Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type Subtask struct {
	Base
	TaskID      uint   `gorm:"index;not null" json:"taskID"`
	Title       string `gorm:"not null" json:"title"`
	IsCompleted bool   `json:"isCompleted"`
}

// ==================== ACTIVITY & STUDY SESSION ====================

type Activity struct {
	Base
	UserID     uint   `gorm:"index;not null" json:"userID"`
	Title      string `gorm:"not null" json:"title"`
	StartTime  string `json:"startTime"`  // e.g., "15:00"
	EndTime    string `json:"endTime"`    // e.g., "17:00"
	Recurrence string `json:"recurrence"` // e.g., "weekly", "daily"

	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type StudySession struct {
	Base
	UserID      uint      `gorm:"index;not null" json:"userID"`
	TaskID      uint      `gorm:"index;not null" json:"taskID"`
	SessionDate time.Time `gorm:"index;not null" json:"sessionDate"`
	StartTime   string    `json:"startTime"` // e.g., "19:00"
	Duration    int       `json:"duration"`  // Dalam menit
	Status      string    `json:"status"`    // planned, completed, skipped
	AIGenerated bool      `json:"aiGenerated"`

	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Task Task `gorm:"foreignKey:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

// ==================== NOTIFICATION & AI PREDICTION ====================

type Notification struct {
	Base
	UserID      uint      `gorm:"index;not null" json:"userID"`
	TaskID      *uint     `gorm:"index" json:"taskID,omitempty"` // Opsional, bisa untuk notifikasi umum
	Title       string    `gorm:"not null" json:"title"`
	Type        string    `json:"type"` // reminder, warning, etc.
	ScheduledAt time.Time `json:"scheduledAt"`
	IsRead      bool      `json:"isRead"`

	User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Task Task `gorm:"foreignKey:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

type MaterialPrediction struct {
	Base
	UserID            uint    `gorm:"index;not null" json:"userID"`
	SubjectID         uint    `gorm:"index;not null" json:"subjectID"`
	PredictedMaterial string  `gorm:"not null" json:"predictedMaterial"`
	Confidence        float64 `gorm:"check:confidence >= 0 AND confidence <= 100" json:"confidence"`
	Reason            string  `json:"reason"`
	UserFeedback      string  `json:"userFeedback,omitempty"` // accurate, inaccurate

	User    User    `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Subject Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

// ==================== MIGRATION HELPERS ====================

func AllModels() []interface{} {
	return []interface{}{
		&User{},
		&UserPreference{},
		&Subject{},
		&Schedule{},
		&Task{},
		&Subtask{},
		&Activity{},
		&StudySession{},
		&Notification{},
		&MaterialPrediction{},
	}
}

func Migrate(db *gorm.DB) error {
	// GORM AutoMigrate sudah aman dipanggil berulang kali.
	// Tidak perlu loop HasTable manual (itu bug di kode lama yang bikin tabel lain skip).
	return db.AutoMigrate(AllModels()...)
}
