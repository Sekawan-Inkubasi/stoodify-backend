# Stoodify Testing Guide

Panduan pengujian otomatis untuk modul backend Stoodify.

---

## 1. Menjalankan Test

Jalankan seluruh test suite dengan perintah:
```bash
go test -v ./...
```

Untuk menguji package tertentu dengan rincian detail:
```bash
go test -v ./internal/priority
```

Untuk memeriksa coverage:
```bash
go test -cover ./...
```

---

## 2. Target Pengujian Utama
- **Priority Scoring Formula (`internal/priority/priority_test.go`)**: deadline dekat → urgensi tinggi, bobot kesulitan/durasi proporsional, progres 100% → skor 0, konsistensi `CalculateBreakdown`.
- **AI bridge parsing (`controllers/ai_controllers_test.go`)**: toleransi bentuk respons AI (wrapper `sessions`/`data`/array polos, alias `session_date`, normalisasi `confidence_score` 0..1 → 0..100, `prerequisites` array vs string) dan `AI_TIMEOUT_SECONDS`.
- **Auth (`controllers/auth_controllers_test.go`)**: integrasi PostgreSQL. **Dilewati otomatis** kalau `TEST_DATABASE_URL` tidak diset — guard ini sengaja agar test tidak menyentuh DB produksi.
- **Kurikulum (`controllers/curriculum_controllers_test.go`)**: sqlite in-memory, tidak butuh PostgreSQL.
- **Token, mailer, rate limiter, routes** (`internal/token`, `internal/mailer`, `internal/ratelimit`, `middlewares/ratelimit_test.go`, `routes/routes_test.go`).

Untuk menjalankan test integrasi auth, set dulu DB terisolasi:
```bash
TEST_DATABASE_URL='postgres://postgres:testpass@localhost:5432/stoodify_test?sslmode=disable' go test ./controllers/ -v
```

---

## 3. Aturan Menulis Unit Test Baru
1. Tempatkan file test berdampingan dengan kode yang diuji menggunakan suffix `_test.go`.
2. Gunakan table-driven tests untuk menguji berbagai variasi input dan edge cases.
3. Jangan gunakan mock yang terlalu rumit jika fungsi murni (*pure function*) dapat diuji langsung; pakai sqlite in-memory untuk logika yang butuh DB, dan JANGAN fallback ke DB produksi.

---

## 4. Gate CI
`dev` memakai `.github/workflows/ci.yml`: `test -z "$(gofmt -l .)"`, `go test -race ./...`, `go vet ./...`, lalu `docker build`. Gate format sengaja `test -z` karena `gofmt -l .` selalu exit 0 walau ada file tidak terformat.
