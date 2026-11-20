package mailer

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// Mailer mendefinisikan antarmuka pengiriman email generik
type Mailer interface {
	SendPasswordReset(toEmail, resetToken string) error
	SendEmailVerification(toEmail, verifyToken string) error
}

// SMTPMailer mengirim email via SMTP standar (STARTTLS pada port 587/25)
type SMTPMailer struct {
	Host            string
	Port            string
	Username        string
	Password        string
	From            string
	BaseURL         string
	VerifyURL       string
	VerificationTTL string
}

// NewMailer membuat Mailer berdasarkan konfigurasi environment
func NewMailer() Mailer {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return &NoopMailer{}
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "no-reply@stoodify.id"
	}
	baseURL := os.Getenv("PASSWORD_RESET_URL")
	if baseURL == "" {
		baseURL = "https://stoodify.id/reset-password"
	}
	verifyURL := os.Getenv("EMAIL_VERIFICATION_URL")
	if verifyURL == "" {
		verifyURL = "https://stoodify.id/verify-email"
	}

	return &SMTPMailer{
		Host:            host,
		Port:            port,
		Username:        os.Getenv("SMTP_USERNAME"),
		Password:        os.Getenv("SMTP_PASSWORD"),
		From:            from,
		BaseURL:         baseURL,
		VerifyURL:       verifyURL,
		VerificationTTL: "24 jam",
	}
}

func (m *SMTPMailer) SendPasswordReset(toEmail, resetToken string) error {
	resetLink := fmt.Sprintf("%s?token=%s", strings.TrimRight(m.BaseURL, "/"), resetToken)
	subject := "Permintaan Reset Password Stoodify"
	body := fmt.Sprintf("Halo,\n\nKami menerima permintaan untuk mereset password akun Stoodify Anda.\nSilakan gunakan link berikut untuk membuat password baru:\n%s\n\nLink ini berlaku selama 15 menit. Jika Anda tidak merasa meminta reset password, abaikan email ini.\n\nSalam,\nTim Stoodify\n", resetLink)
	return m.kirim(toEmail, subject, body)
}

func (m *SMTPMailer) SendEmailVerification(toEmail, verifyToken string) error {
	link := fmt.Sprintf("%s?token=%s", strings.TrimRight(m.VerifyURL, "/"), verifyToken)
	ttl := m.VerificationTTL
	if ttl == "" {
		ttl = "24 jam"
	}
	subject := "Verifikasi Email Stoodify"
	body := fmt.Sprintf("Halo,\n\nTerima kasih sudah mendaftar di Stoodify.\nKlik link berikut untuk memverifikasi email dan mengaktifkan akun Anda:\n%s\n\nLink ini berlaku selama %s. Jika Anda tidak merasa mendaftar, abaikan email ini.\n\nSalam,\nTim Stoodify\n", link, ttl)
	return m.kirim(toEmail, subject, body)
}

// kirim menyusun pesan teks sederhana lalu mengirimkannya lewat SMTP.
func (m *SMTPMailer) kirim(toEmail, subject, body string) error {
	headers := "From: " + m.From + "\r\n" +
		"To: " + toEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-version: 1.0;\r\n" +
		"Content-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	msg := []byte(headers + body)

	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)
	var auth smtp.Auth
	if m.Username != "" || m.Password != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	if err := smtp.SendMail(addr, auth, m.From, []string{toEmail}, msg); err != nil {
		// Jangan pernah mencatat token, cukup alamat & jenis kegagalannya.
		log.Printf("mailer: gagal mengirim %q ke %s: %v", subject, toEmail, err)
		return err
	}
	return nil
}

// NoopMailer digunakan saat dev/testing atau jika SMTP_HOST tidak diset
type NoopMailer struct{}

func (n *NoopMailer) SendPasswordReset(toEmail, resetToken string) error {
	// Tidak mengirim ke jaringan luar, tidak mencetak token ke log
	return nil
}

func (n *NoopMailer) SendEmailVerification(toEmail, verifyToken string) error {
	return nil
}
