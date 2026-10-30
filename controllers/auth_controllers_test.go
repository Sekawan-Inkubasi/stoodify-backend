package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

func setupTestDB(t *testing.T) *gorm.DB {
	os.Setenv("JWT_SECRET", "test_jwt_secret_key_1234567890")

	// Wajibkan TEST_DATABASE_URL khusus terisolasi. Dilarang memuat .env atau fallback ke POSTGRES_URL/DATABASE_URL produksi.
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("[GUARD] TEST_DATABASE_URL tidak disetel. Tes integrasi PostgreSQL dilewati demi keamanan agar tidak menyentuh database aplikasi.")
		return nil
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("[TEST DB] Gagal terhubung ke TEST_DATABASE_URL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("[TEST DB] Gagal inisialisasi sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	if err := db.AutoMigrate(&migrations.User{}, &migrations.PasswordResetToken{}, &migrations.RefreshToken{}); err != nil {
		t.Fatalf("[TEST DB] Gagal AutoMigrate: %v", err)
	}
	return db
}

func TestPasswordResetConfirm_ConcurrentSingleUse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	ctrl := NewAuthController(db)

	// Buat user dummy dengan email unik
	userEmail := fmt.Sprintf("student1_%d@example.com", time.Now().UnixNano())
	user := migrations.User{
		Email:        userEmail,
		PasswordHash: "$2a$14$test",
		TokenVersion: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("gagal create user: %v", err)
	}

	rawToken, hash, _ := token.GenerateRandomToken()
	resetRecord := migrations.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := db.Create(&resetRecord).Error; err != nil {
		t.Fatalf("gagal create reset record: %v", err)
	}

	r := gin.New()
	r.POST("/api/auth/password-reset/confirm", ctrl.PasswordResetConfirm)

	concurrentReqs := 10
	var wg sync.WaitGroup
	var successCount int32
	var failCount int32

	wg.Add(concurrentReqs)
	for i := 0; i < concurrentReqs; i++ {
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{
				"token":            rawToken,
				"password":         "PasswordBaru123!",
				"confirm_password": "PasswordBaru123!",
			})
			req, _ := http.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("harus tepat 1 request confirm yang berhasil, dapat: %d", successCount)
	}
	if failCount != int32(concurrentReqs-1) {
		t.Errorf("sisa request harus gagal (400), dapat: %d gagal dari %d total", failCount, concurrentReqs)
	}

	// Verifikasi bahwa user token_version naik dan token reset terisi used_at
	var updatedToken migrations.PasswordResetToken
	db.First(&updatedToken, resetRecord.ID)
	if updatedToken.UsedAt == nil {
		t.Errorf("used_at seharusnya tidak nil setelah sukses digunakan")
	}

	var updatedUser migrations.User
	db.First(&updatedUser, user.ID)
	if updatedUser.TokenVersion != 2 {
		t.Errorf("token_version user seharusnya naik jadi 2, dapat: %d", updatedUser.TokenVersion)
	}
}

func TestRefreshToken_ConcurrentRotation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	ctrl := NewAuthController(db)

	userEmail := fmt.Sprintf("student2_%d@example.com", time.Now().UnixNano())
	user := migrations.User{
		Email:        userEmail,
		PasswordHash: "$2a$14$test",
		TokenVersion: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("gagal create user: %v", err)
	}

	// Buat refresh token aktif berversi 1
	refreshIssued, err := token.GenerateRefresh(user.ID, 1)
	if err != nil {
		t.Fatalf("gagal generate refresh token: %v", err)
	}
	refRecord := migrations.RefreshToken{
		UserID:    user.ID,
		TokenHash: token.Hash(refreshIssued.Token),
		ExpiresAt: refreshIssued.ExpiresAt,
	}
	if err := db.Create(&refRecord).Error; err != nil {
		t.Fatalf("gagal create refresh record: %v", err)
	}

	r := gin.New()
	r.POST("/api/auth/refresh", ctrl.Refresh)

	concurrentReqs := 10
	var wg sync.WaitGroup
	var successCount int32
	var failCount int32

	wg.Add(concurrentReqs)
	for i := 0; i < concurrentReqs; i++ {
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{
				"refresh_token": refreshIssued.Token,
			})
			req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if w.Code == http.StatusUnauthorized {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("harus tepat 1 request refresh yang berhasil, dapat: %d", successCount)
	}
	if failCount != int32(concurrentReqs-1) {
		t.Errorf("sisa request refresh harus ditolak 401, dapat: %d", failCount)
	}
}

func TestRefreshToken_TokenVersionMismatchRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	ctrl := NewAuthController(db)

	// User dengan token_version = 2 (setelah reset/ganti password)
	userEmail := fmt.Sprintf("student3_%d@example.com", time.Now().UnixNano())
	user := migrations.User{
		Email:        userEmail,
		PasswordHash: "$2a$14$test",
		TokenVersion: 2,
	}
	db.Create(&user)

	// Refresh token lama dibuat dengan token_version = 1
	oldRefresh, _ := token.GenerateRefresh(user.ID, 1)
	refRecord := migrations.RefreshToken{
		UserID:    user.ID,
		TokenHash: token.Hash(oldRefresh.Token),
		ExpiresAt: oldRefresh.ExpiresAt,
	}
	db.Create(&refRecord)

	r := gin.New()
	r.POST("/api/auth/refresh", ctrl.Refresh)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": oldRefresh.Token,
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("refresh token dengan token_version basi harus ditolak 401, got: %d", w.Code)
	}
}

func TestPasswordResetRequest_GenericResponseEquivalence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	ctrl := NewAuthController(db)

	registeredEmail := fmt.Sprintf("exists_%d@example.com", time.Now().UnixNano())
	user := migrations.User{
		Email:        registeredEmail,
		PasswordHash: "$2a$14$test",
		TokenVersion: 1,
	}
	db.Create(&user)

	r := gin.New()
	r.POST("/api/auth/password-reset/request", ctrl.PasswordResetRequest)

	// Test email ada
	body1, _ := json.Marshal(map[string]string{"email": registeredEmail})
	req1, _ := http.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	// Test email tidak ada
	unregisteredEmail := fmt.Sprintf("notfound_%d@example.com", time.Now().UnixNano())
	body2, _ := json.Marshal(map[string]string{"email": unregisteredEmail})
	req2, _ := http.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// 1. Verifikasi respons publik dan kode status identik (mencegah enumerasi akun)
	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("kedua request harus 200 OK, got: %d dan %d", w1.Code, w2.Code)
	}
	if w1.Body.String() != w2.Body.String() {
		t.Errorf("respons publik harus identik secara karakter, w1: %s, w2: %s", w1.Body.String(), w2.Body.String())
	}

	t.Logf("[GENERIC RESPONSE CHECK] Kedua alur sukses mengembalikan envelope dan pesan publik identik")
}
