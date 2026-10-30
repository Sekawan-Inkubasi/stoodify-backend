package migrations

import (
	"time"

	"gorm.io/gorm"
)

// ==================== ENUM CONSTANTS ====================

const (
	RoleStudent = "student"
	RoleTeacher = "teacher"
	RoleAdmin   = "admin"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusLate       = "late"
)

const (
	SessionPlanned   = "planned"
	SessionCompleted = "completed"
	SessionSkipped   = "skipped"
)

const (
	NotifReminder = "reminder"
	NotifWarning  = "warning"
	NotifLate     = "late"
)

const (
	FeedbackAccurate   = "accurate"
	FeedbackInaccurate = "inaccurate"
)

// ==================== BASE MODEL ====================

type Base struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ==================== USER & PREFERENCES ====================

type User struct {
	Base
	Name         string          `gorm:"not null" json:"name"`
	Email        string          `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string          `gorm:"column:password_hash;not null" json:"-"`
	Role         string          `gorm:"default:student" json:"role"`
	School       string          `gorm:"type:varchar(150);" json:"school"`
	Grade        int8            `gorm:"type:smallint" json:"grade"`
	Major        string          `gorm:"type:varchar(100)" json:"major"`
	TokenVersion int             `gorm:"default:1;not null" json:"-"`
	Preferences  *UserPreference `gorm:"foreignKey:UserID" json:"preferences,omitempty"`
}

type UserPreference struct {
	Base
	UserID             uint   `gorm:"uniqueIndex;not null" json:"user_id"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	SessionDuration    int    `json:"session_duration"`
	ActiveDays         string `json:"active_days"`
	BreakDuration      int    `gorm:"default:15" json:"break_duration"`
	MaxSessionsPerDay  int    `gorm:"default:3" json:"max_sessions_per_day"`
}

// ==================== SUBJECT & SCHEDULE ====================

type Subject struct {
	Base
	UserID     uint   `gorm:"index;not null" json:"user_id"`
	Name       string `gorm:"not null" json:"name"`
	Code       string `gorm:"type:varchar(20)" json:"code"`
	Teacher    string `gorm:"type:varchar(100)" json:"teacher"`
	Color      string `json:"color"`
	AccentBg   string `gorm:"type:varchar(30)" json:"accent_bg"`
	Category   string `gorm:"type:varchar(50)" json:"category"`
	Difficulty int    `gorm:"check:difficulty >= 1 AND difficulty <= 5" json:"difficulty"`
}

type Schedule struct {
	Base
	UserID    uint   `gorm:"index;not null" json:"user_id"`
	SubjectID uint   `gorm:"index" json:"subject_id"`
	DayOfWeek string `json:"day_of_week"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Room      string `gorm:"type:varchar(50)" json:"room"`
	Teacher   string `gorm:"type:varchar(100)" json:"teacher"`

	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"subject,omitempty"`
}

// ==================== TASK & SUBTASK ====================

type Task struct {
	Base
	UserID            uint      `gorm:"index;not null" json:"user_id"`
	SubjectID         uint      `gorm:"index" json:"subject_id"`
	Title             string    `gorm:"not null" json:"title"`
	MaterialName      string    `gorm:"type:varchar(100)" json:"material_name"`
	Description       string    `json:"description"`
	Type              string    `gorm:"default:assignment" json:"type"` // exam, project, assignment, reading
	Deadline          time.Time `gorm:"index;not null" json:"deadline"`
	Difficulty        int       `gorm:"check:difficulty >= 1 AND difficulty <= 5" json:"difficulty"`
	EstimatedDuration int       `gorm:"default:60" json:"estimated_duration"` // menit
	PriorityScore     float64   `json:"priority_score"`
	Status            string    `gorm:"index;default:pending" json:"status"`
	Progress          int       `gorm:"default:0;check:progress >= 0 AND progress <= 100" json:"progress"`
	IsSnoozed         bool      `gorm:"default:false" json:"is_snoozed"`
	Notes             string    `json:"notes"`
	Subtasks          []Subtask `gorm:"foreignKey:TaskID" json:"subtasks,omitempty"`

	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"subject,omitempty"`
}

type Subtask struct {
	Base
	TaskID      uint   `gorm:"index;not null" json:"task_id"`
	Title       string `gorm:"not null" json:"title"`
	IsCompleted bool   `json:"is_completed"`
}

// ==================== ACTIVITY & STUDY SESSION ====================

type Activity struct {
	Base
	UserID     uint   `gorm:"index;not null" json:"user_id"`
	Title      string `gorm:"not null" json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Recurrence string `json:"recurrence"`
}

type StudySession struct {
	Base
	UserID         uint      `gorm:"index;not null" json:"user_id"`
	TaskID         uint      `gorm:"index;not null" json:"task_id"`
	SessionDate    time.Time `gorm:"index;not null" json:"session_date"`
	StartTime      string    `json:"start_time"`
	Duration       int       `json:"duration"`
	TargetProgress int       `json:"target_progress"`
	Status         string    `gorm:"default:planned" json:"status"`
	AIGenerated    bool      `json:"ai_generated"`
	Reason         string    `json:"reason"`

	Task *Task `gorm:"foreignKey:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"task,omitempty"`
}

// ==================== NOTIFICATION & AI PREDICTION ====================

type Notification struct {
	Base
	UserID      uint      `gorm:"index;not null" json:"user_id"`
	TaskID      *uint     `gorm:"index" json:"task_id,omitempty"`
	Title       string    `gorm:"not null" json:"title"`
	Message     string    `json:"message"`
	Type        string    `json:"type"` // reminder, warning, late
	ScheduledAt time.Time `gorm:"index" json:"scheduled_at"`
	IsRead      bool      `json:"is_read"`
}

type MaterialPrediction struct {
	Base
	UserID            uint    `gorm:"index;not null" json:"user_id"`
	SubjectID         uint    `gorm:"index;not null" json:"subject_id"`
	PredictedMaterial string  `gorm:"not null" json:"predicted_material"`
	Confidence        float64 `gorm:"check:confidence >= 0 AND confidence <= 100" json:"confidence"`
	Reason            string  `json:"reason"`
	Prerequisites     string  `json:"prerequisites"`
	Preparation       string  `json:"preparation_suggestion"`
	UserFeedback      string  `json:"user_feedback,omitempty"`

	Subject *Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"subject,omitempty"`
}

// ==================== AUTH ====================

type RefreshToken struct {
	Base
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type PasswordResetToken struct {
	Base
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	User      User       `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time `gorm:"index" json:"used_at,omitempty"`
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
		&RefreshToken{},
		&PasswordResetToken{},
	}
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(AllModels()...); err != nil {
		return err
	}
	return TambahIndexKinerja(db)
}

// TambahIndexKinerja: index untuk query terpanas (deadline, status, user_id).
// AutoMigrate tidak membuat index komposit, jadi dibuat eksplisit di sini.
func TambahIndexKinerja(db *gorm.DB) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_status ON tasks (user_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_deadline ON tasks (user_id, deadline)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_user_status_deadline ON tasks (user_id, status, deadline)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_user_day ON schedules (user_id, day_of_week)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_date ON study_sessions (user_id, session_date)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications (user_id, is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_task_type ON notifications (task_id, type, scheduled_at)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_active ON refresh_tokens (user_id, revoked_at)`,
		`CREATE INDEX IF NOT EXISTS idx_predictions_user_subject ON material_predictions (user_id, subject_id)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// DropAll menghapus semua tabel. Urutan dibalik (anak -> induk) supaya FK aman.
func DropAll(db *gorm.DB) error {
	models := AllModels()
	for i := len(models) - 1; i >= 0; i-- {
		if err := db.Migrator().DropTable(models[i]); err != nil {
			return err
		}
	}
	return nil
}
