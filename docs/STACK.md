# Stoodify Tech Stack & Environment

Ringkasan teknologi, dependensi, dan batasan lingkungan runtime Stoodify Backend.

---

## 1. Core Stack
- **Bahasa**: Go 1.27 (lihat `go.mod`; Dockerfile & CI memakai `golang:1.27-alpine`)
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin) (`v1.12.0`)
- **ORM & Database Driver**: [GORM](https://gorm.io) (`v1.31.2`) dengan driver PostgreSQL (`gorm.io/driver/postgres v1.6.3`)
- **Database**: PostgreSQL (Host: Supabase / Neon Cloud Database)
- **Autentikasi**: JWT ([golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) `v5.3.1`) dengan algoritma HS256
- **Hashing Password**: Bcrypt (`golang.org/x/crypto/bcrypt`, cost 14)

---

## 2. Struktur Direktori
```
stoodify-backend/
├── docs/AGENTS.md            # Entry point dan panduan agen AI
├── README.md                 # Gambaran umum & panduan lokal
├── main.go                   # CLI (serve/migrate/seed) + bootstrapping server
├── config/                   # Koneksi GORM + env loader (storage S3 masih unused)
├── controllers/              # Layer HTTP request/response handling
├── docs/                     # Dokumentasi arsitektur, API, ADR, dan panduan
├── internal/
│   ├── hashing_password/     # Wrapper fungsi hashing bcrypt
│   ├── mailer/               # Pengirim email reset password (SMTP / Noop)
│   ├── params/               # Parser query params & pagination
│   ├── priority/             # Algoritma Priority Scoring & Unit Tests
│   ├── ratelimit/            # Store limiter in-memory
│   ├── response/             # Response envelope standar (OK, Fail, Error)
│   ├── seeder/               # Database seeder (sintetis + akun demo)
│   └── token/                # Generator dan validator JWT
├── middlewares/              # RequireAuth, RequireRole (belum dipakai), CORS, Rate Limiter
├── migrations/               # Model GORM dan skema AutoMigrate
├── routes/                   # Routing table endpoint REST
└── workers/                  # Background worker (late task updater & reminder)
```

---

## 3. Perintah Kerja Standar
- **Download Modul**: `go mod download`
- **Jalankan Server**: `go run . serve` (atau `go run .`, default serve)
- **Jalankan Seeder**: `go run . seed` / `go run . seed --demo`
- **Jalankan Unit Test**: `go test ./...`
- **Cek Lint / Vet**: `go vet ./...` dan `test -z "$(gofmt -l .)"` (gate yang dipakai CI)
- **Format Kode**: `gofmt -w -s .`
