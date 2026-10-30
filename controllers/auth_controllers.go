package controllers

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	hashingPassword "stoodify_backend/internal/hashing_password"
	"stoodify_backend/internal/mailer"
	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"
)

// bcrypt dari string acak; dipakai saat email tidak ketemu agar waktu respons
// setara dengan kasus email ada (cegah user enumeration via timing).
var dummyHash = "$2a$14$YI1f5208nuCQceNMwRc1OuaQ5DiHWckmIKqvVDTDFk4LKh6/v/AmW"

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type AuthController struct {
	DB     *gorm.DB
	Mailer mailer.Mailer
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{
		DB:     db,
		Mailer: mailer.NewMailer(),
	}
}

type RegisterInput struct {
	Name            string `json:"name" binding:"required,min=1,max=100"`
	Email           string `json:"email" binding:"required,email,max=100"`
	School          string `json:"school" binding:"required,max=150"`
	Grade           int8   `json:"grade" binding:"required,min=7,max=13"`
	Major           string `json:"major" binding:"required,max=100"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type PasswordResetRequestInput struct {
	Email string `json:"email" binding:"required,email"`
}

type PasswordResetConfirmInput struct {
	Token           string `json:"token" binding:"required"`
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	OldPassword     string `json:"old_password"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password"`
}

func (ctrl *AuthController) Register(konteks *gin.Context) {
	var input RegisterInput
	var existing migrations.User

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if input.Password != input.ConfirmPassword {
		response.UnprocessableEntity(konteks, "password dan konfirmasi password tidak cocok", map[string]string{
			"confirm_password": "password mismatch",
		})
		return
	}

	input.Email = normalizeEmail(input.Email)

	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ?", input.Email).First(&existing).Error
	if err == nil {
		response.Fail(konteks, 409, "email sudah terdaftar, harap login alih alih registrasi", map[string]string{
			"body": "email registered",
		})
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		response.ServerError(konteks, err)
		return
	}

	hashed, err := hashingPassword.HashPassword(input.Password)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	user := migrations.User{
		Name:         input.Name,
		School:       input.School,
		Grade:        input.Grade,
		Major:        input.Major,
		Email:        input.Email,
		PasswordHash: hashed,
		Role:         migrations.RoleStudent,
		TokenVersion: 1,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&user).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.Created(konteks, "registrasi berhasil, sekarang harap login", user)
}

func (ctrl *AuthController) Login(konteks *gin.Context) {
	var input LoginInput
	var user migrations.User

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	email := normalizeEmail(input.Email)
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ?", email).First(&user).Error
	// email tidak ketemu & password salah dibalas pesan identik (401), supaya
	// penyerang tidak bisa menebak email mana yang terdaftar.
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.ServerError(konteks, err)
			return
		}
		hashingPassword.CheckPasswordHash(input.Password, dummyHash)
		response.Unauthorized(konteks, "email atau password salah")
		return
	}

	if !hashingPassword.CheckPasswordHash(input.Password, user.PasswordHash) {
		response.Unauthorized(konteks, "email atau password salah")
		return
	}

	tokenVer := user.TokenVersion
	if tokenVer <= 0 {
		tokenVer = 1
	}

	access, refresh, err := ctrl.issueTokens(konteks, user.ID, tokenVer)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "login berhasil", gin.H{
		"access_token":  access.Token,
		"refresh_token": refresh.Token,
		"token_type":    "Bearer",
		"expires_in":    int(token.AccessTTL.Seconds()),
		"user":          user,
	})
}

// Refresh menukar refresh token lama dengan pasangan token baru (rotasi).
func (ctrl *AuthController) Refresh(konteks *gin.Context) {
	var input RefreshInput

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	userID, _, tokenVerClaim, err := token.Parse(input.RefreshToken, token.TypeRefresh)
	if err != nil {
		response.Unauthorized(konteks, "refresh token tidak valid atau sudah kedaluwarsa")
		return
	}

	var newAccess, newRefresh token.Issued
	now := time.Now()
	tokenHash := token.Hash(input.RefreshToken)

	err = ctrl.DB.WithContext(konteks.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// 1. Secara atomik cabut refresh token lama dengan syarat belum dicabut dan belum expired
		res := tx.Model(&migrations.RefreshToken{}).
			Where("token_hash = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, userID, now).
			Update("revoked_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("token_revoked_or_invalid")
		}

		// 2. Ambil data user dalam transaksi untuk validasi token_version
		var user migrations.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		userTokenVer := user.TokenVersion
		if userTokenVer <= 0 {
			userTokenVer = 1
		}

		// Jika versi token tidak sesuai (misal telah direset/diganti password), tolak rotasi
		if tokenVerClaim != 0 && tokenVerClaim != userTokenVer {
			return errors.New("token_version_mismatch")
		}

		// 3. Terbitkan access & refresh token baru yang terikat token_version user
		access, err := token.GenerateAccess(user.ID, userTokenVer)
		if err != nil {
			return err
		}
		refresh, err := token.GenerateRefresh(user.ID, userTokenVer)
		if err != nil {
			return err
		}

		// 4. Catat refresh token baru di database
		newRecord := migrations.RefreshToken{
			UserID:    user.ID,
			TokenHash: token.Hash(refresh.Token),
			ExpiresAt: refresh.ExpiresAt,
		}
		if err := tx.Create(&newRecord).Error; err != nil {
			return err
		}

		newAccess = access
		newRefresh = refresh
		return nil
	})

	if err != nil {
		if err.Error() == "token_revoked_or_invalid" || err.Error() == "token_version_mismatch" {
			response.Unauthorized(konteks, "refresh token sudah tidak berlaku")
			return
		}
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "token diperbarui", gin.H{
		"access_token":  newAccess.Token,
		"refresh_token": newRefresh.Token,
		"token_type":    "Bearer",
		"expires_in":    int(token.AccessTTL.Seconds()),
	})
}

// Logout mencabut refresh token milik user (idempotent).
func (ctrl *AuthController) Logout(konteks *gin.Context) {
	user := middlewares.MustUser(konteks)

	var input RefreshInput
	// Body opsional: kalau dikirim, cabut token itu saja.
	if err := konteks.ShouldBindJSON(&input); err == nil && input.RefreshToken != "" {
		ctrl.DB.WithContext(konteks.Request.Context()).
			Model(&migrations.RefreshToken{}).
			Where("user_id = ? AND token_hash = ? AND revoked_at IS NULL", user.ID, token.Hash(input.RefreshToken)).
			Update("revoked_at", time.Now())
		response.OK(konteks, "logout berhasil", nil)
		return
	}

	ctrl.DB.WithContext(konteks.Request.Context()).
		Model(&migrations.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", user.ID).
		Update("revoked_at", time.Now())

	response.OK(konteks, "logout berhasil", nil)
}

func (ctrl *AuthController) Me(konteks *gin.Context) {
	response.OK(konteks, "data user", middlewares.MustUser(konteks))
}

// PasswordResetRequest menangani permintaan reset password publik
func (ctrl *AuthController) PasswordResetRequest(konteks *gin.Context) {
	var input PasswordResetRequestInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	email := normalizeEmail(input.Email)
	var user migrations.User
	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ?", email).First(&user).Error

	// Selalu kembalikan respons generik yang sama apakah email terdaftar atau tidak (mencegah enumerasi akun)
	genericMsg := "Jika email terdaftar, instruksi reset password telah dikirimkan ke email Anda."

	rawToken, hash, errGen := token.GenerateRandomToken()
	if errGen != nil {
		response.ServerError(konteks, errGen)
		return
	}
	if err == nil {
		// Simpan hanya hash token di database dengan masa berlaku 15 menit
		resetRecord := migrations.PasswordResetToken{
			UserID:    user.ID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&resetRecord).Error; err == nil {
			// Kirim via mailer secara async (token mentah tidak pernah dicatat di log atau respons)
			if ctrl.Mailer != nil {
				go func() {
					_ = ctrl.Mailer.SendPasswordReset(user.Email, rawToken)
				}()
			}
		}
	} else {
		// Jalur simetris: jalankan dummy DB query dan goroutine tiruan
		// agar beban kerja, round-trip DB, dan goroutine kedua jalur seimbang
		var dummy migrations.PasswordResetToken
		_ = ctrl.DB.WithContext(konteks.Request.Context()).Where("token_hash = ?", hash).First(&dummy).Error
		go func() {
			_ = rawToken
		}()
	}

	response.OK(konteks, genericMsg, nil)
}

// PasswordResetConfirm mereset password menggunakan token yang valid dan belum terpakai
func (ctrl *AuthController) PasswordResetConfirm(konteks *gin.Context) {
	var input PasswordResetConfirmInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if input.Password != input.ConfirmPassword {
		response.UnprocessableEntity(konteks, "password dan konfirmasi password tidak cocok", map[string]string{
			"confirm_password": "password mismatch",
		})
		return
	}

	tokenHash := token.Hash(input.Token)
	now := time.Now()

	err := ctrl.DB.WithContext(konteks.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var resetRecord migrations.PasswordResetToken
		if err := tx.Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, now).First(&resetRecord).Error; err != nil {
			return errors.New("token_invalid")
		}

		// Tandai token reset telah digunakan secara atomik dengan syarat used_at IS NULL
		res := tx.Model(&migrations.PasswordResetToken{}).
			Where("id = ? AND used_at IS NULL", resetRecord.ID).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("token_invalid")
		}

		// Hash password baru dengan bcrypt cost 14
		hashed, err := hashingPassword.HashPassword(input.Password)
		if err != nil {
			return err
		}

		// Update password hash dan naikkan token_version agar access token lama invalid
		if err := tx.Model(&migrations.User{}).Where("id = ?", resetRecord.UserID).
			Updates(map[string]interface{}{
				"password_hash": hashed,
				"token_version": gorm.Expr("token_version + 1"),
			}).Error; err != nil {
			return err
		}

		// Cabut seluruh refresh token aktif milik user
		if err := tx.Model(&migrations.RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", resetRecord.UserID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if err.Error() == "token_invalid" {
			response.BadRequest(konteks, "token reset password tidak valid atau sudah kedaluwarsa", nil)
			return
		}
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "password berhasil direset, silakan login dengan password baru", nil)
}

// ChangePassword mengganti password untuk user terautentikasi dan membatalkan seluruh sesi lama
func (ctrl *AuthController) ChangePassword(konteks *gin.Context) {
	var input ChangePasswordInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	// Dukung alias old_password atau current_password
	oldPass := input.CurrentPassword
	if oldPass == "" {
		oldPass = input.OldPassword
	}
	if oldPass == "" {
		response.BadRequest(konteks, "password saat ini wajib diisi", map[string]string{
			"current_password": "password saat ini wajib diisi",
		})
		return
	}

	if input.ConfirmPassword != "" && input.NewPassword != input.ConfirmPassword {
		response.UnprocessableEntity(konteks, "password baru dan konfirmasi password tidak cocok", map[string]string{
			"confirm_password": "password mismatch",
		})
		return
	}

	user := middlewares.MustUser(konteks)

	// Verifikasi password saat ini
	if !hashingPassword.CheckPasswordHash(oldPass, user.PasswordHash) {
		response.BadRequest(konteks, "password saat ini salah", map[string]string{
			"current_password": "password salah",
		})
		return
	}

	hashed, err := hashingPassword.HashPassword(input.NewPassword)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	now := time.Now()
	err = ctrl.DB.WithContext(konteks.Request.Context()).Transaction(func(tx *gorm.DB) error {
		// Update password & increment token_version
		if err := tx.Model(&migrations.User{}).Where("id = ?", user.ID).
			Updates(map[string]interface{}{
				"password_hash": hashed,
				"token_version": gorm.Expr("token_version + 1"),
			}).Error; err != nil {
			return err
		}

		// Cabut seluruh refresh token aktif
		if err := tx.Model(&migrations.RefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", user.ID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "password berhasil diubah, seluruh sesi lama telah dicabut", nil)
}

func (ctrl *AuthController) issueTokens(konteks *gin.Context, userID uint, tokenVersion int) (token.Issued, token.Issued, error) {
	access, err := token.GenerateAccess(userID, tokenVersion)
	if err != nil {
		return token.Issued{}, token.Issued{}, err
	}
	refresh, err := token.GenerateRefresh(userID, tokenVersion)
	if err != nil {
		return token.Issued{}, token.Issued{}, err
	}

	record := migrations.RefreshToken{
		UserID:    userID,
		TokenHash: token.Hash(refresh.Token),
		ExpiresAt: refresh.ExpiresAt,
	}
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&record).Error; err != nil {
		return token.Issued{}, token.Issued{}, err
	}
	return access, refresh, nil
}
