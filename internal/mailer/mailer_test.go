package mailer

import (
	"testing"
)

func TestNewMailer_DefaultNoop(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	m := NewMailer()
	if _, ok := m.(*NoopMailer); !ok {
		t.Errorf("expected *NoopMailer when SMTP_HOST is empty, got %T", m)
	}
	if err := m.SendPasswordReset("user@example.com", "dummy-token"); err != nil {
		t.Errorf("expected nil error from NoopMailer, got %v", err)
	}
}

func TestNewMailer_SMTPConfigured(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_FROM", "admin@stoodify.id")
	t.Setenv("PASSWORD_RESET_URL", "https://stoodify.id/auth/reset")

	m := NewMailer()
	smtpM, ok := m.(*SMTPMailer)
	if !ok {
		t.Fatalf("expected *SMTPMailer when SMTP_HOST is set, got %T", m)
	}
	if smtpM.Host != "smtp.example.com" || smtpM.Port != "2525" {
		t.Errorf("unexpected host/port: %s:%s", smtpM.Host, smtpM.Port)
	}
	if smtpM.From != "admin@stoodify.id" {
		t.Errorf("unexpected from: %s", smtpM.From)
	}
	if smtpM.BaseURL != "https://stoodify.id/auth/reset" {
		t.Errorf("unexpected baseURL: %s", smtpM.BaseURL)
	}
}
