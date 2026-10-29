# Stoodify Backend

REST API Stoodify — asisten belajar harian berbasis kecerdasan buatan (AI) untuk siswa SMP, SMA, dan SMK di Indonesia. Dibangun dengan performa tinggi menggunakan Golang, Gin Web Framework, GORM, dan PostgreSQL (Supabase / Neon).

---

## 🚀 Panduan Menjalankan

### 1. Prasyarat & Setup Lingkungan
Pastikan Anda telah menginstal Go versi 1.22+ dan PostgreSQL.
```bash
# Salin file konfigurasi environment
cp .env.example .env
# Isi kredensial POSTGRES_URL dan JWT_SECRET di dalam .env

# Unduh dependensi modul
go mod download
```

Tabel dan indeks database akan dibuat secara otomatis saat boot aplikasi melalui mekanisme GORM `AutoMigrate`.

### 2. Menjalankan Server API
```bash
go run main.go serve       # Berjalan pada http://localhost:8080
```

### 3. Mengisi Data Awal (Seeder Demo)
Untuk keperluan pengujian atau demo pameran, tersedia seeder data realistis (siswa, mapel, jadwal, tugas, sesi belajar, dan admin):
```bash
go run main.go seed        # Membuat data demo untuk 10 user uji
```
Akun default demo:
- **Siswa Uji**: `siswa.uji@stoodify.seed` / `rahasia123`
- **Admin**: `admin@stoodify.seed` / `rahasia123`

---

## 📁 Struktur Direktori

```
stoodify-backend/
├── main.go            # Bootstrapping server, router, CLI commands (serve & seed)
├── config/            # Inisialisasi koneksi GORM ke PostgreSQL
├── controllers/       # HTTP Request Handlers (Bind, Validate, Controller logic)
├── routes/            # Pendaftaran route API & middleware groups
├── middlewares/       # RequireAuth (JWT Bearer), RequireRole (RBAC), CORS
├── migrations/        # Model database GORM, tipe data, skema migrasi otomatis
├── workers/           # Scheduler background: auto-late task updater & reminder
├── internal/
│   ├── hashing_password/ # Hashing password menggunakan bcrypt (cost 14)
│   ├── params/           # Parser parameter URL dan pagination helper
│   ├── priority/         # Priority Scoring Engine & Unit Tests formula PRD
│   ├── response/         # Response envelope standar (OK, Created, Fail, Error)
│   ├── seeder/           # Script seeder data uji realistis
│   └── token/            # Generator & validator JWT Access Token dan Refresh Token
└── docs/              # Dokumentasi lengkap arsitektur, API, ADR, dan panduan agen
```

---

## 📚 Dokumentasi Lengkap

- **Panduan untuk AI Agent**: [`docs/AGENTS.md`](docs/AGENTS.md)
- **Katalog Lengkap Endpoint API**: [`docs/API.md`](docs/API.md)
- **Kontrak Integrasi AI**: [`docs/AI_CONTRACT.md`](docs/AI_CONTRACT.md)
- **Arsitektur Sistem**: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- **Catatan Keputusan Desain (ADR)**: [`docs/DECISIONS.md`](docs/DECISIONS.md)
- **Tech Stack & Direktori**: [`docs/STACK.md`](docs/STACK.md)
- **Konvensi Kode**: [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md)
- **Panduan Pengujian (Testing)**: [`docs/TESTING.md`](docs/TESTING.md)
- **Troubleshooting & Known Errors**: [`docs/ERRORS.md`](docs/ERRORS.md)
- **Status & Serah Terima**: [`docs/STATE.md`](docs/STATE.md)
- **Daftar Tugas & Backlog**: [`docs/to-do.md`](docs/to-do.md)
- **Panduan Deployment Production**: [`docs/DEPLOY.md`](docs/DEPLOY.md)

---

## 🧪 Verifikasi & Uji Kode

Pastikan seluruh linter, unit test, dan kompilasi bersih sebelum melakukan commit:
```bash
go test -v ./... && go vet ./... && go build ./...
```
