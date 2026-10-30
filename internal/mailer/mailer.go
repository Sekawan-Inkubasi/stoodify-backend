package mailer

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// Mailer mendefinisikan antarmuka pengiriman email generik
type Mailer interface {
	SendPasswordReset(toEmail, resetToken string) error
}

// SMTPMailer mengirim email via SMTP standar
type SMTPMailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	BaseURL  string
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

	return &SMTPMailer{
		Host:     host,
		Port:     port,
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
		BaseURL:  baseURL,
	}
}

func (m *SMTPMailer) SendPasswordReset(toEmail, resetToken string) error {
	resetLink := fmt.Sprintf("%s?token=%s", strings.TrimRight(m.BaseURL, "/"), resetToken)
	subject := "Subject: Permintaan Reset Password Stoodify\r\n"
	mime := "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	body := fmt.Sprintf("Halo,\n\nKami menerima permintaan untuk mereset password akun Stoodify Anda.\nSilakan gunakan link berikut untuk membuat password baru:\n%s\n\nLink ini berlaku selama 15 menit. Jika Anda tidak merasa meminta reset password, abaikan email ini.\n\nSalam,\nTim Stoodify\n", resetLink)
	msg := []byte(subject + mime + body)

	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)
	var auth smtp.Auth
	if m.Username != "" || m.Password != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	return smtp.SendMail(addr, auth, m.From, []string{toEmail}, msg)
}

// NoopMailer digunakan saat dev/testing atau jika SMTP_HOST tidak diset
type NoopMailer struct{}

func (n *NoopMailer) SendPasswordReset(toEmail, resetToken string) error {
	// Tidak mengirim ke jaringan luar, tidak mencetak token ke log
	return nil
}
