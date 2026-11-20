package controllers

import (
	"errors"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/googleauth"
	"stoodify_backend/internal/response"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

// GoogleLoginInput: id_token didapat aplikasi dari Google Sign-In SDK
// (Android/iOS/web). Backend memverifikasinya sendiri; jangan pernah percaya
// email/nama yang dikirim client secara terpisah.
type GoogleLoginInput struct {
	IDToken string `json:"id_token" binding:"required"`
}

// LoginGoogle menukar Google ID token menjadi token aplikasi.
//
// Alur:
//   - token diverifikasi (signature, issuer, audience, exp, email_verified)
//   - dicari berdasarkan google_sub; kalau belum ada, dicocokkan lewat email
//     terverifikasi (auto-link) atau dibuat akun baru tanpa password
//   - token aplikasi SELALU diterbitkan, dengan penanda needs_onboarding bila
//     sekolah/kelas/jurusan belum diisi
func (ctrl *AuthController) LoginGoogle(konteks *gin.Context) {
	var input GoogleLoginInput
	if err := konteks.ShouldBindJSON(&input); err != nil {
		response.ValidationError(konteks, err)
		return
	}
	ctrl.ProsesLoginGoogle(konteks, input.IDToken)
}

// ProsesLoginGoogle memverifikasi ID token Google dan menerbitkan token aplikasi.
func (ctrl *AuthController) ProsesLoginGoogle(konteks *gin.Context, idToken string) {
	verifier := ctrl.Google
	if verifier == nil {
		verifier = googleauth.NewVerifier(os.Getenv("GOOGLE_CLIENT_ID"))
	}

	klaim, err := verifier.Verify(konteks.Request.Context(), idToken)
	if err != nil {
		if errors.Is(err, googleauth.ErrClientIDKosong) {
			response.Fail(konteks, 503, "login Google belum dikonfigurasi di server", nil)
			return
		}
		// Satu pesan untuk semua kegagalan verifikasi: jangan bocorkan alasan teknis.
		response.Unauthorized(konteks, "token Google tidak valid atau sudah kedaluwarsa")
		return
	}

	user, dibuat, err := ctrl.userDariGoogle(konteks, klaim)
	if err != nil {
		var bentrok *emailBentrokError
		if errors.As(err, &bentrok) {
			response.Fail(konteks, 409, bentrok.Error(), nil)
			return
		}
		response.ServerError(konteks, err)
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

	perluOnboarding := user.AuthProvider == migrations.AuthProviderGoogle && !user.ProfileCompleted
	payload := gin.H{
		"access_token":     access.Token,
		"refresh_token":    refresh.Token,
		"token_type":       "Bearer",
		"expires_in":       int(token.GetAccessTTL().Seconds()),
		"user":             user,
		"needs_onboarding": perluOnboarding,
		"auth_provider":    user.AuthProvider,
	}

	if dibuat {
		response.Created(konteks, "akun berhasil dibuat dengan Google, lengkapi profil untuk mulai belajar", payload)
		return
	}
	response.OK(konteks, "login berhasil", payload)
}

type emailBentrokError struct{ pesan string }

func (e *emailBentrokError) Error() string { return e.pesan }

// userDariGoogle mencari/membuat user dari klaim Google yang sudah terverifikasi.
func (ctrl *AuthController) userDariGoogle(konteks *gin.Context, klaim googleauth.Claims) (migrations.User, bool, error) {
	ctx := konteks.Request.Context()
	var user migrations.User

	// 1. Sudah pernah login Google sebelumnya (identitas paling kuat: sub).
	err := ctrl.DB.WithContext(ctx).Where("google_sub = ?", klaim.Sub).First(&user).Error
	if err == nil {
		ctrl.lengkapiDariGoogle(&user, klaim)
		return user, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return migrations.User{}, false, err
	}

	// 2. Email sudah terdaftar sebagai akun password -> auto-link (email sudah
	//    terverifikasi Google, jadi kepemilikan email terbukti).
	err = ctrl.DB.WithContext(ctx).Where("email = ?", klaim.Email).First(&user).Error
	if err == nil {
		if user.GoogleSub != nil && *user.GoogleSub != klaim.Sub {
			return migrations.User{}, false, &emailBentrokError{
				pesan: "email ini sudah terhubung ke akun Google lain",
			}
		}
		sub := klaim.Sub
		user.GoogleSub = &sub
		// Email dari Google sudah terverifikasi Google sendiri.
		user.EmailVerified = true
		ctrl.lengkapiDariGoogle(&user, klaim)
		if err := ctrl.DB.WithContext(ctx).Save(&user).Error; err != nil {
			return migrations.User{}, false, err
		}
		return user, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return migrations.User{}, false, err
	}

	// 3. Akun baru: tanpa password, profil belum lengkap.
	user = migrations.User{
		Name:             namaDariGoogle(klaim),
		Email:            klaim.Email,
		GoogleSub:        &klaim.Sub,
		AuthProvider:     migrations.AuthProviderGoogle,
		ProfileCompleted: false,
		EmailVerified:    true,
		AvatarURL:        klaim.Picture,
		Role:             migrations.RoleStudent,
		TokenVersion:     1,
	}
	if err := ctrl.DB.WithContext(ctx).Create(&user).Error; err != nil {
		// Balapan dua request login bersamaan: ambil baris yang sudah dibuat.
		var ulang migrations.User
		if e := ctrl.DB.WithContext(ctx).Where("google_sub = ?", klaim.Sub).First(&ulang).Error; e == nil {
			return ulang, false, nil
		}
		return migrations.User{}, false, err
	}
	// Kolom profile_completed punya DEFAULT true (supaya data lama aman), sehingga
	// GORM mengabaikan nilai false saat INSERT. Set eksplisit setelah insert.
	if err := ctrl.DB.WithContext(ctx).Model(&migrations.User{}).
		Where("id = ?", user.ID).Update("profile_completed", false).Error; err != nil {
		return migrations.User{}, false, err
	}
	user.ProfileCompleted = false
	return user, true, nil
}

// lengkapiDariGoogle hanya mengisi data yang masih kosong; data yang sudah
// diubah user sendiri tidak ditimpa.
func (ctrl *AuthController) lengkapiDariGoogle(user *migrations.User, klaim googleauth.Claims) {
	if user.AvatarURL == "" && klaim.Picture != "" {
		user.AvatarURL = klaim.Picture
	}
	if strings.TrimSpace(user.Name) == "" {
		user.Name = namaDariGoogle(klaim)
	}
}

func namaDariGoogle(klaim googleauth.Claims) string {
	if nama := strings.TrimSpace(klaim.Name); nama != "" {
		return nama
	}
	if idx := strings.Index(klaim.Email, "@"); idx > 0 {
		return klaim.Email[:idx]
	}
	return "Siswa Stoodify"
}
