package controllers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/mailer"
	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

type UserController struct {
	DB     *gorm.DB
	Mailer mailer.Mailer
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{
		DB:     db,
		Mailer: mailer.NewMailer(),
	}
}

type UpdateMeInput struct {
	Name   *string `json:"name" binding:"omitempty,min=1,max=100"`
	Email  *string `json:"email" binding:"omitempty,email,max=100"`
	School *string `json:"school" binding:"omitempty,max=150"`
	Grade  *int8   `json:"grade" binding:"omitempty,min=10,max=13"`
	Major  *string `json:"major" binding:"omitempty,max=100"`
}

type PreferencesInput struct {
	StartTime         string  `json:"start_time" binding:"omitempty,len=5"`
	EndTime           string  `json:"end_time" binding:"omitempty,len=5"`
	SessionDuration   int     `json:"session_duration" binding:"omitempty,min=15,max=600"`
	ActiveDays        string  `json:"active_days"`
	BreakDuration     int     `json:"break_duration" binding:"omitempty,min=5,max=120"`
	MaxSessionsPerDay int     `json:"max_sessions_per_day" binding:"omitempty,min=1,max=10"`
	Goals             *string `json:"goals"`
	SchoolStart       *string `json:"school_start"`
	SchoolEnd         *string `json:"school_end"`
}

// OnboardingInput: data wajib yang harus dilengkapi akun Google sebelum fitur
// belajar (jadwal AI, prediksi, statistik) terbuka.
type OnboardingInput struct {
	School string `json:"school" binding:"required,max=150"`
	Grade  int8   `json:"grade" binding:"required,min=10,max=13"`
	Major  string `json:"major" binding:"required,max=100"`
}

func (ctrl *UserController) GetMe(konteks *gin.Context) {
	response.OK(konteks, "data user", currentUser(konteks))
}

// CompleteOnboarding melengkapi sekolah/kelas/jurusan lalu membuka fitur belajar.
// Idempotent: bisa dipanggil ulang untuk memperbarui data yang sama.
func (ctrl *UserController) CompleteOnboarding(konteks *gin.Context) {
	user := currentUser(konteks)

	var input OnboardingInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	user.School = strings.TrimSpace(input.School)
	user.Grade = input.Grade
	user.Major = strings.TrimSpace(input.Major)
	user.ProfileCompleted = true

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&user).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	picuRegenerasi(user.ID)
	response.OK(konteks, "profil belajar lengkap, semua fitur sudah terbuka", user)
}

func (ctrl *UserController) UpdateMe(konteks *gin.Context) {
	user := currentUser(konteks)
	var input UpdateMeInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if input.School != nil {
		user.School = *input.School
	}

	if input.Grade != nil {
		user.Grade = *input.Grade
	}

	if input.Major != nil {
		user.Major = *input.Major
	}

	if input.Name != nil {
		user.Name = *input.Name
	}

	emailBerubah := false
	if input.Email != nil {
		email := normalizeEmail(*input.Email)
		if email != user.Email {
			var other migrations.User
			if err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ? AND id <> ?", email, user.ID).First(&other).Error; err == nil {
				response.Fail(konteks, 409, "email sudah dipakai user lain", nil)
				return
			}
			user.Email = email
			user.EmailVerified = false
			emailBerubah = true
		}
	}

	ctx := konteks.Request.Context()
	if err := ctrl.DB.WithContext(ctx).Save(&user).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	if emailBerubah {
		// Pastikan email_verified = false tercatat di DB dan kirim link verifikasi ke email baru.
		if err := ctrl.DB.WithContext(ctx).Model(&migrations.User{}).
			Where("id = ?", user.ID).Update("email_verified", false).Error; err != nil {
			response.ServerError(konteks, err)
			return
		}
		raw, hash, err := token.GenerateRandomToken()
		if err != nil {
			log.Printf("update-me: gagal membuat token verifikasi untuk %s: %v", user.Email, err)
		} else {
			now := time.Now()
			txErr := ctrl.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&migrations.EmailVerificationToken{}).
					Where("user_id = ? AND used_at IS NULL", user.ID).
					Update("used_at", now).Error; err != nil {
					return err
				}
				return tx.Create(&migrations.EmailVerificationToken{
					UserID:    user.ID,
					TokenHash: hash,
					ExpiresAt: now.Add(TTLVerifikasiEmail),
				}).Error
			})
			if txErr != nil {
				log.Printf("update-me: gagal menyimpan token verifikasi untuk %s: %v", user.Email, txErr)
			} else if ctrl.Mailer != nil {
				go func(targetEmail, verifyToken string) {
					if err := ctrl.Mailer.SendEmailVerification(targetEmail, verifyToken); err != nil {
						log.Printf("update-me: gagal kirim email verifikasi ke %s: %v", targetEmail, err)
					}
				}(user.Email, raw)
			}
		}
		response.OK(konteks, "profil diperbarui, silakan verifikasi email baru Anda", user)
		return
	}

	response.OK(konteks, "profil diperbarui", user)
}

func (ctrl *UserController) GetPreferences(konteks *gin.Context) {
	user := currentUser(konteks)

	var pref migrations.UserPreference
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("user_id = ?", user.ID).First(&pref).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.OK(konteks, "preferensi belum diatur", migrations.UserPreference{UserID: user.ID})
		return
	}
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, "preferensi user", pref)
}

func (ctrl *UserController) UpdatePreferences(konteks *gin.Context) {
	user := currentUser(konteks)
	var input PreferencesInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	var pref migrations.UserPreference
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("user_id = ?", user.ID).First(&pref).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ServerError(konteks, err)
		return
	}

	pref.UserID = user.ID
	if input.StartTime != "" {
		pref.StartTime = input.StartTime
	}
	if input.EndTime != "" {
		pref.EndTime = input.EndTime
	}
	if input.SessionDuration > 0 {
		pref.SessionDuration = input.SessionDuration
	}
	if input.ActiveDays != "" {
		pref.ActiveDays = input.ActiveDays
	}
	if input.BreakDuration > 0 {
		pref.BreakDuration = input.BreakDuration
	}
	if input.MaxSessionsPerDay > 0 {
		pref.MaxSessionsPerDay = input.MaxSessionsPerDay
	}
	if input.Goals != nil {
		pref.Goals = *input.Goals
	}
	if input.SchoolStart != nil {
		pref.SchoolStart = *input.SchoolStart
	}
	if input.SchoolEnd != nil {
		pref.SchoolEnd = *input.SchoolEnd
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Save(&pref).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}
	picuRegenerasi(user.ID)
	response.OK(konteks, "preferensi diperbarui", pref)
}

func (ctrl *UserController) UploadAvatar(konteks *gin.Context) {
	user := currentUser(konteks)

	file, header, err := konteks.Request.FormFile("avatar")
	if err != nil {
		response.Fail(konteks, 400, "file avatar wajib diunggah", nil)
		return
	}
	defer file.Close()

	if header.Size > 2*1024*1024 {
		response.Fail(konteks, 400, "ukuran file avatar maksimal 2MB", nil)
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		response.Fail(konteks, 400, "format avatar harus JPG, JPEG, PNG, atau WEBP", nil)
		return
	}

	uploadDir := "uploads/avatars"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		response.ServerError(konteks, err)
		return
	}

	filename := fmt.Sprintf("avatar_%d_%d%s", user.ID, time.Now().Unix(), ext)
	targetPath := filepath.Join(uploadDir, filename)

	out, err := os.Create(targetPath)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		response.ServerError(konteks, err)
		return
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)
	user.AvatarURL = avatarURL

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Model(&user).Update("avatar_url", avatarURL).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "avatar berhasil diperbarui", gin.H{
		"avatar_url": avatarURL,
	})
}
