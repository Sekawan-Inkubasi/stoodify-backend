package controllers

import (
	//"log"
	//"net/smtp"
	//"net/textproto"
	//"os"
	"errors"
	"strings"

	"stoodify_backend/internal/hashing_password"
	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/middlewares"
	"stoodify_backend/migrations"

	"github.com/gin-gonic/gin"

	//"github.com/jordan-wright/email"
	"gorm.io/gorm"
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
	Password        string `json:"password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (ctrl *AuthController) Register(konteks *gin.Context) {
	var input RegisterInput
	var user migrations.User
	var payload migrations.User

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.BadRequest(konteks, "Body request tidak valid", map[string]string{
			"body": err.Error(),
		})
		return
	}

	//password validation
	if input.Password != input.ConfirmPassword {
		response.UnprocessableEntity(konteks, "Input password dan konfirmasi password salah password dan konfirmasi password tidak cocok", map[string]string{
			"body": "password missmatch",
		})
		return
	}

	input.Email = normalizeEmail(input.Email)

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ?", input.Email).First(&user).Error; err != nil && errors.Is(err, gorm.ErrRecordNotFound) == false {
		response.Fail(konteks, 500, "mohon maaf ada kesalahan di sisi kami", map[string]string{
			"body": err.Error(),
		})
		return
	} else if err == nil {
		response.Fail(konteks, 409, "email sudah terdaftar, harap login alih alih registrasi", map[string]string{
			"body": "email registered",
		})
		return
	}

	// password sanitation

	if hashed_password, err := hashingPassword.HashPassword(input.Password); err != nil {
		response.Fail(konteks, 500, "mohon maaf ada kesalahan di sisi kami", map[string]string{
			"body": err.Error(),
		})
		return
	} else {
		payload = migrations.User{
			Name:         input.Name,
			Email:        input.Email,
			PasswordHash: hashed_password,
		}
	}

	if err := ctrl.DB.WithContext(konteks.Request.Context()).Create(&payload).Error; err != nil {
		response.Fail(konteks, 500, "gagal membuat user", map[string]string{
			"body": err.Error(),
		})
		return
	}

	response.Created(konteks, "registrasi berhasil, sekarang harap login", payload)
	return
	/*
		response.Created(konteks, "Registrasi berhasil!", map[string]string{
			"Name": input.Name,
			"Email": input.Email,
		})
		return
	*/

	//email verification
	/*
		email_admin := os.Getenv("SMTP_USER")
		email_pass := os.Getenv("SMTP_PASS")
		email_host := os.Getenv("SMTP_HOST")
		e := &email.Email{
			To:      []string{input.Email},
			From:    fmt.Sprintf("Admin Stoodify <%s>", email_admin),
			Subject: "Verifikasi Pembuatan akun Stoodify",
			HTML:    []byte("<h1>abc</h1>"),
			Headers: textproto.MIMEHeader{},
		}

		if err := e.Send(fmt.Sprintf("%s:587", email_host), smtp.PlainAuth("", email_admin, email_pass, email_host)); err == nil {
			response.OK(konteks, "wokeh udah terkirim, coba cek!", map[string]string{
				"adalah": "adalahhhh",
			})
			return
		} else {
			response.Fail(konteks, 400, "gagal", map[string]string{
				"adalah": "oioioi",
			})
			log.Fatalf("gagal nih %v", err)
			return
		}
	*/
	// etc
}

func (ctrl *AuthController) Login(konteks *gin.Context) {
	var input LoginInput
	var user migrations.User

	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.BadRequest(konteks, "Body request tidak valid", map[string]string{
			"body": err.Error(),
		})
		return
	}

	email := normalizeEmail(input.Email)

	err := ctrl.DB.WithContext(konteks.Request.Context()).Where("email = ?", email).First(&user).Error

	// email tidak ketemu & password salah dibalas pesan identik (401), supaya
	// penyerang tidak bisa menebak email mana yang terdaftar.
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(konteks, 500, "mohon maaf ada kesalahan di sisi kami", map[string]string{
				"body": err.Error(),
			})
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

	accessToken, err := token.Generate(user.ID)
	if err != nil {
		response.Fail(konteks, 500, "mohon maaf ada kesalahan di sisi kami", map[string]string{
			"body": err.Error(),
		})
		return
	}

	response.OK(konteks, "login berhasil", map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(token.ExpireDuration.Seconds()),
		"user":         user,
	})
}

// Me — memastikan token masih valid; butuh middleware RequireAuth.
func (ctrl *AuthController) Me(konteks *gin.Context) {
	user, ok := middlewares.CurrentUser(konteks)
	if !ok {
		response.Unauthorized(konteks, "token tidak valid")
		return
	}
	response.OK(konteks, "data user", user)
}
