package controllers

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

// TTLVerifikasiEmail: masa berlaku link verifikasi.
const TTLVerifikasiEmail = 24 * time.Hour

type VerifyEmailInput struct {
	Token string `json:"token" binding:"required"`
}

type ResendVerificationInput struct {
	Email string `json:"email" binding:"required,email"`
}

// VerifyEmail mengaktifkan akun setelah user mengeklik link di email.
// Token sekali pakai dan punya masa berlaku 24 jam.
func (ctrl *AuthController) VerifyEmail(konteks *gin.Context) {
	var input VerifyEmailInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	ctx := konteks.Request.Context()
	hash := token.Hash(input.Token)

	var record migrations.EmailVerificationToken
	err := ctrl.DB.WithContext(ctx).Where("token_hash = ?", hash).First(&record).Error
	if err != nil {
		response.BadRequest(konteks, "token verifikasi tidak valid atau sudah dipakai", map[string]string{
			"token": "tidak valid",
		})
		return
	}
	if record.UsedAt != nil {
		response.BadRequest(konteks, "token verifikasi tidak valid atau sudah dipakai", map[string]string{
			"token": "sudah dipakai",
		})
		return
	}
	if time.Now().After(record.ExpiresAt) {
		response.BadRequest(konteks, "token verifikasi sudah kedaluwarsa, silakan minta kirim ulang", map[string]string{
			"token": "kedaluwarsa",
		})
		return
	}

	now := time.Now()
	err = ctrl.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Tandai token terpakai hanya kalau belum pernah dipakai (proteksi request ganda).
		res := tx.Model(&migrations.EmailVerificationToken{}).
			Where("id = ? AND used_at IS NULL", record.ID).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("token_sudah_dipakai")
		}
		return tx.Model(&migrations.User{}).Where("id = ?", record.UserID).
			Update("email_verified", true).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "token_sudah_dipakai" {
			response.BadRequest(konteks, "token verifikasi tidak valid atau sudah dipakai", map[string]string{
				"token": "sudah dipakai",
			})
			return
		}
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "email berhasil diverifikasi, sekarang kamu bisa login", gin.H{
		"email_verified": true,
	})
}

// ResendVerification mengirim ulang link verifikasi. Balasannya selalu sama
// (tidak membocorkan apakah email terdaftar atau sudah terverifikasi).
func (ctrl *AuthController) ResendVerification(konteks *gin.Context) {
	var input ResendVerificationInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	pesan := "Jika email terdaftar dan belum diverifikasi, link verifikasi baru sudah dikirim."
	ctx := konteks.Request.Context()

	var user migrations.User
	err := ctrl.DB.WithContext(ctx).Where("email = ?", normalizeEmail(input.Email)).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(konteks, err)
			return
		}
		response.OK(konteks, pesan, nil)
		return
	}

	// Sudah terverifikasi atau akun Google: tidak ada yang perlu dikirim.
	if user.EmailVerified || user.AuthProvider == migrations.AuthProviderGoogle {
		response.OK(konteks, pesan, nil)
		return
	}

	if err := ctrl.buatDanKirimVerifikasi(ctx, user); err != nil {
		response.ServerError(konteks, err)
		return
	}
	response.OK(konteks, pesan, nil)
}

// buatDanKirimVerifikasi membuat token baru (membatalkan token lama yang belum
// dipakai) lalu mengirim email di latar belakang. Kegagalan kirim hanya dicatat
// di log: user tetap bisa minta kirim ulang.
func (ctrl *AuthController) buatDanKirimVerifikasi(ctx context.Context, user migrations.User) error {
	raw, hash, err := token.GenerateRandomToken()
	if err != nil {
		return err
	}

	now := time.Now()
	err = ctrl.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	if err != nil {
		return err
	}

	if ctrl.Mailer != nil {
		go func(email string) {
			if err := ctrl.Mailer.SendEmailVerification(email, raw); err != nil {
				log.Printf("verifikasi email: gagal kirim ke %s: %v", email, err)
			}
		}(user.Email)
	}
	return nil
}
