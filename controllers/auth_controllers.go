package controllers

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	hashingPassword "stoodify_backend/internal/hashing_password"
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
	DB *gorm.DB
}

func NewAuthController(db *gorm.DB) *AuthController {
	return &AuthController{DB: db}
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

func (ctrl *AuthController) Register(konteks *gin.Context) {
	var input RegisterInput
	var existing migrations.User

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}

	if input.Password != input.ConfirmPassword {
		response.UnprocessableEntity(konteks, "password dan konfirmasi password tidak cocok", map[string]string{
			"confirm_password": "password missmatch",
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

	access, refresh, err := ctrl.issueTokens(konteks, user.ID)
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

	userID, _, err := token.Parse(input.RefreshToken, token.TypeRefresh)
	if err != nil {
		response.Unauthorized(konteks, "refresh token tidak valid atau sudah kedaluwarsa")
		return
	}

	var stored migrations.RefreshToken
	err = ctrl.DB.WithContext(konteks.Request.Context()).
		Where("token_hash = ?", token.Hash(input.RefreshToken)).First(&stored).Error
	if err != nil || stored.RevokedAt != nil || stored.UserID != userID || stored.ExpiresAt.Before(time.Now()) {
		response.Unauthorized(konteks, "refresh token sudah tidak berlaku")
		return
	}

	var user migrations.User
	if err := ctrl.DB.WithContext(konteks.Request.Context()).First(&user, userID).Error; err != nil {
		response.Unauthorized(konteks, "user dari token tidak ditemukan")
		return
	}

	// Rotasi: token lama langsung dicabut; kalau gagal, jangan terbitkan yang baru.
	now := time.Now()
	if err := ctrl.DB.WithContext(konteks.Request.Context()).Model(&stored).Update("revoked_at", now).Error; err != nil {
		response.ServerError(konteks, err)
		return
	}

	access, refresh, err := ctrl.issueTokens(konteks, user.ID)
	if err != nil {
		response.ServerError(konteks, err)
		return
	}

	response.OK(konteks, "token diperbarui", gin.H{
		"access_token":  access.Token,
		"refresh_token": refresh.Token,
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

func (ctrl *AuthController) issueTokens(konteks *gin.Context, userID uint) (token.Issued, token.Issued, error) {
	access, err := token.GenerateAccess(userID)
	if err != nil {
		return token.Issued{}, token.Issued{}, err
	}
	refresh, err := token.GenerateRefresh(userID)
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
