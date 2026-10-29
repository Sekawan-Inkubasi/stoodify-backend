# Stoodify Tech Stack & Environment

Ringkasan teknologi, dependensi, dan batasan lingkungan runtime Stoodify Backend.

---

## 1. Core Stack
- **Bahasa**: Go 1.22+ (terverifikasi kompatibel s.d. Go 1.24)
- **Web Framework**: [Gin Web Framework](https://github.com/gin-gonic/gin) (`v1.10.0`)
- **ORM & Database Driver**: [GORM](https://gorm.io) (`v1.25.12`) dengan driver PostgreSQL (`gorm.io/driver/postgres v1.5.11`)
- **Database**: PostgreSQL (Host: Supabase / Neon Cloud Database)
- **Autentikasi**: JWT ([golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) `v5.2.1`) dengan algoritma HS256
- **Hashing Password**: Bcrypt (`golang.org/x/crypto/bcrypt`, default cost: 14)

---

## 2. Struktur Direktori
```
stoodify-backend/
├── AGENTS.md                 # Entry point dan panduan agen AI
├── README.md                 # Gambaran umum & panduan lokal
├── main.go                   # Bootstrapping server & graceful shutdown
├── config/                   # Inisialisasi database GORM
├── controllers/              # Layer HTTP request/response handling
├── docs/                     # Dokumentasi arsitektur, API, ADR, dan panduan
├── internal/
│   ├── hashing_password/     # Wrapper fungsi hashing bcrypt
│   ├── params/               # Parser query params & pagination
│   ├── priority/             # Algoritma Priority Scoring & Unit Tests
│   ├── response/             # Response envelope standar (OK, Fail, Error)
│   ├── seeder/               # Database seeder untuk demo/testing
│   └── token/                # Generator dan validator JWT
├── middlewares/              # Auth, RBAC, CORS middleware
├── migrations/               # Model GORM dan skema AutoMigrate
├── routes/                   # Routing table endpoint REST
└── workers/                  # Background worker (late task updater & reminders)
```

---

## 3. Perintah Kerja Standar
- **Download Modul**: `go mod download`
- **Jalankan Server**: `go run main.go serve` (atau `go run .`)
- **Jalankan Seeder**: `go run main.go seed`
- **Jalankan Unit Test**: `go test -v ./...`
- **Cek Lint / Vet**: `go vet ./...`
- **Format Kode**: `gofmt -w -s .`
