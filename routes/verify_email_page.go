package routes

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"stoodify_backend/internal/token"
	"stoodify_backend/migrations"
)

// Skema deep link yang dibuka halaman verifikasi untuk meneruskan token ke aplikasi.
const defaultDeepLinkScheme = "stoodify://verify-email"

// halamanVerifikasi adalah jembatan https -> deep link.
//
// Link di email HARUS https (Gmail/Outlook memblokir klik `stoodify://`), jadi
// halaman ini membuka aplikasi lewat deep link yang membawa token. Verifikasi
// sesungguhnya tetap dilakukan aplikasi lewat POST /api/auth/verify-email.
func halamanVerifikasi(db *gorm.DB) gin.HandlerFunc {
	return func(konteks *gin.Context) {
		rawToken := strings.TrimSpace(konteks.Query("token"))

		konteks.Header("Referrer-Policy", "no-referrer")
		konteks.Header("Cache-Control", "no-store")
		konteks.Header("X-Robots-Tag", "noindex")

		if rawToken == "" {
			konteks.Data(http.StatusBadRequest, "text/html; charset=utf-8",
				[]byte(halamanStatus("Link tidak lengkap",
					"Token verifikasi tidak ditemukan pada link ini. Buka ulang tautan dari email terbaru.", "", false)))
			return
		}

		status, judul, pesan := statusTokenVerifikasi(db, rawToken)

		deepLink := ""
		if status == "valid" {
			deepLink = deepLinkScheme() + "?token=" + url.QueryEscape(rawToken)
		}

		konteks.Data(http.StatusOK, "text/html; charset=utf-8",
			[]byte(halamanStatus(judul, pesan, deepLink, status == "valid")))
	}
}

// statusTokenVerifikasi mengecek token tanpa mengubah data. Verifikasi tetap
// dilakukan aplikasi, supaya membuka link dua kali tidak "menghabiskan" token
// (email client sering mem-prefetch link).
func statusTokenVerifikasi(db *gorm.DB, rawToken string) (status, judul, pesan string) {
	var record migrations.EmailVerificationToken
	if err := db.Where("token_hash = ?", token.Hash(rawToken)).First(&record).Error; err != nil {
		return "invalid", "Link tidak dikenali",
			"Link verifikasi ini tidak valid. Minta kirim ulang dari aplikasi."
	}
	if record.UsedAt != nil {
		return "used", "Email sudah terverifikasi",
			"Email kamu sudah diverifikasi sebelumnya. Silakan login di aplikasi."
	}
	if time.Now().After(record.ExpiresAt) {
		return "expired", "Link sudah kedaluwarsa",
			"Link verifikasi berlaku 24 jam. Minta kirim ulang dari aplikasi untuk mendapatkan link baru."
	}
	return "valid", "Verifikasi email kamu",
		"Tekan tombol di bawah untuk membuka aplikasi Stoodify dan menyelesaikan verifikasi."
}

func deepLinkScheme() string {
	if v := strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_DEEPLINK")); v != "" {
		return v
	}
	return defaultDeepLinkScheme
}

// halamanStatus merender satu halaman HTML minimal (tanpa aset eksternal).
func halamanStatus(judul, pesan, deepLink string, valid bool) string {
	tombol := `<p class="hint">Buka aplikasi Stoodify lalu masuk seperti biasa.</p>`
	if valid && deepLink != "" {
		// Tombol sekaligus auto-redirect: banyak pengguna hanya butuh satu klik.
		tombol = `<a class="btn" href="` + htmlEscape(deepLink) + `">Buka Aplikasi Stoodify</a>
  <p class="hint">Kalau aplikasi tidak terbuka otomatis, tekan tombol di atas.</p>
  <script>setTimeout(function(){location.href="` + jsEscape(deepLink) + `";}, 600);</script>`
	}

	return `<!doctype html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>` + htmlEscape(judul) + ` — Stoodify</title>
  <style>
    :root { color-scheme: dark; }
    body { margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center;
           background:#0b1120; color:#e2e8f0; font-family:system-ui,-apple-system,"Segoe UI",sans-serif; }
    .card { max-width:420px; padding:32px 24px; text-align:center; }
    h1 { font-size:1.35rem; margin:0 0 12px; }
    p { color:#94a3b8; line-height:1.55; margin:0 0 20px; }
    .btn { display:inline-block; padding:14px 22px; border-radius:12px; background:#6366f1;
           color:#fff; text-decoration:none; font-weight:600; }
    .hint { font-size:.85rem; color:#64748b; margin-top:16px; }
  </style>
</head>
<body>
  <main class="card">
    <h1>` + htmlEscape(judul) + `</h1>
    <p>` + htmlEscape(pesan) + `</p>
    ` + tombol + `
  </main>
</body>
</html>`
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func jsEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "'", `\'`, "<", `\u003c`, ">", `\u003e`, "&", `\u0026`)
	return r.Replace(s)
}
