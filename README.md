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

### 2. Pilihan Cara Menjalankan CLI (Go Run vs Binary Hasil Compile)

Backend Stoodify menyediakan binary mandiri dan skrip CLI yang fleksibel. Anda dapat menjalankannya dengan dua cara:

#### Cara A: Menggunakan `go run .` (Development / Hot-edit)
Kompilasi otomatis ke cache temporer Go saat perintah dipanggil:
```bash
go run . serve                # Menjalankan server API pada port default (8080)
go run . seed --demo          # Menanamkan akun demo Muhammad Naufal Rafa Al As'ad
go run . migrate:fresh --demo # Hapus semua tabel, migrasi ulang, langsung isi akun demo
```

#### Cara B: Menggunakan Binary File Hasil Kompilasi
Di root folder backend terdapat binary executable:
- **`./stoodify`**: Binary CLI resmi hasil kompilasi terbaru (`go build -o stoodify .`).
- **`./stoodify_backend`**: Binary build legacy yang sebelumnya dihasilkan otomatis mengikuti nama module `module stoodify_backend` di `go.mod`.

Keduanya adalah executable binary Linux (ELF 64-bit). Untuk pemakaian harian dan demo, disarankan menggunakan `./stoodify`:
```bash
./stoodify serve                # Start server
./stoodify migrate:fresh --demo # Reset total database & pasang akun demo
./stoodify seed --demo          # Pasang/refresh akun demo saja
./stoodify help                 # Bantuan CLI
```

---

## 🛠️ Manajemen Database & Akun Demo

Untuk demonstrasi aplikasi secara cepat dan langsung menguji fitur AI Scheduler maupun Priority Engine:

### 1. Reset Total Database & Pasang Akun Demo (Fresh Setup)
Jika ingin mengosongkan database dari awal (menghapus semua tabel lama) dan langsung mengisinya dengan data demo lengkap:
```bash
go run . migrate:fresh --demo
# atau
./stoodify migrate:fresh --demo
```

### 2. Kredensial Akun Demonstrasi (`seed --demo`)
Data akun demo yang ditanamkan khusus untuk siswa SMK jurusan SIJA:
- **Nama Siswa**: `Muhammad Naufal Rafa Al As'ad`
- **Email**: `naufal@student.id`
- **Password**: `password123`
- **Sekolah**: `SMK Telkom Sidoarjo` (Kelas 12, Jurusan Sistem Informasi, Jaringan, Aplikasi)
- **Preferensi AI**: Belajar mandiri pukul 16:00 - 21:00 WIB (Durasi 60m, Istirahat 15m, Maks 3 sesi/hari).
- **Mata Pelajaran**: 15 mata pelajaran lengkap dengan guru, kategori kejuruan/umum, dan warna tema mobile.
- **Jadwal Pelajaran**: Senin s.d. Jumat (07:00 - 15:10 WIB) sesuai jadwal riil kelas XII SIJA 2 (membantu AI mendeteksi slot belajar kosong di sore/malam hari).
- **Daftar Tugas Aktif**: 8 tugas produktif (Project IoT, Praktikum SKJ2 Hardening, Cerkak B. Jawa, Pohon Keputusan KA2, Cloudflare Workers PaaS, dll.) yang langsung terhitung Priority Score-nya.

### 3. Seeder Massal untuk Stresstest / Mobile Dev Umum
Jika ingin membuat puluhan akun sintetis acak (`@stoodify.seed`):
```bash
go run . seed --users=50 --tasks=40 --reset
```

---

## 📁 Struktur Direktori

```
stoodify-backend/
├── main.go            # Bootstrapping server, router, CLI commands (serve, migrate, seed)
├── stoodify           # Binary executable utama hasil compile
├── stoodify_backend   # Binary executable legacy (mengikuti nama go module)
├── config/            # Inisialisasi koneksi GORM ke PostgreSQL
├── controllers/       # HTTP Request Handlers (Bind, Validate, Controller logic)
├── routes/            # Pendaftaran route API & middleware groups
├── middlewares/       # RequireAuth (JWT Bearer), RequireRole (RBAC), CORS, Rate Limiter
├── migrations/        # Model database GORM, tipe data, skema migrasi otomatis
├── workers/           # Scheduler background: auto-late task updater & reminder
├── internal/
│   ├── hashing_password/ # Hashing password menggunakan bcrypt (cost 14)
│   ├── params/           # Parser parameter URL dan pagination helper
│   ├── priority/         # Priority Scoring Engine & Unit Tests formula PRD
│   ├── response/         # Response envelope standar (OK, Created, Fail, Error)
│   ├── seeder/           # Seeder massal sintetis dan seeder khusus akun demo (demo.go)
│   └── token/            # Generator & validator JWT Access Token dan Refresh Token
└── docs/              # Dokumentasi lengkap arsitektur, API, ADR, dan panduan agen
```

---

## 📚 Panduan Membaca Dokumentasi (Wajib untuk Semua Developer)

Semua developer dan AI Agent yang berkontribusi pada repository ini **sangat disarankan membaca dokumen-dokumen berikut** sesuai fokus tugasnya:

### 🌟 Rekomendasi Utama (Wajib Dibaca Pertama Kali)
1. **[`docs/CONTEXT.md`](docs/CONTEXT.md)**: **Konteks Bisnis & Target Pengguna.** Memahami visi Stoodify untuk siswa SMP/SMA/SMK Indonesia, alur demonstrasi, dan timeline inkubasi.
2. **[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)**: **Arsitektur Sistem & Domain Boundaries.** Memahami pemisahan layer, multi-tenant isolation berbasis `user_id`, serta siklus hidup request.
3. **[`docs/API.md`](docs/API.md)**: **Katalog Lengkap REST API.** Kontrak endpoint publik, header Authorization Bearer, format payload, upload avatar, dan respons JSON.
4. **[`docs/AI_CONTRACT.md`](docs/AI_CONTRACT.md)**: **Kontrak Integrasi AI Engine.** Spesifikasi payload bridge antara backend Go dan microservice Python FastAPI (`stoodify-ai-1`) untuk Adaptive Rescheduling.

### 📐 Standar Koding & Database
5. **[`docs/CONVENTIONS.md`](docs/CONVENTIONS.md)**: **Konvensi Kode & Response Format.** Aturan standard error response, Gin binding tag, validasi input, dan struktur controller.
6. **[`docs/DECISIONS.md`](docs/DECISIONS.md)**: **Architecture Decision Records (ADR).** Alasan mengapa keputusan teknis tertentu diambil (misal: perhitungan dinamis `priority_breakdown`, format OpenCollection Bruno, dll).
7. **[`docs/STACK.md`](docs/STACK.md)**: **Detail Tech Stack.** Versi Go, PostgreSQL, GORM, library pendukung, dan batasan dependensi luar.
8. **[`docs/SEED.md`](docs/SEED.md)**: **Panduan Seeder & Data Sintetis.** Struktur pembuatan data uji, formula prioritas seeder, dan akun demo.

### 🛡️ Keamanan, Pengujian & Troubleshooting
9. **[`docs/SECURITY.md`](docs/SECURITY.md)**: **Standar Keamanan.** Sanitasi database internal error (pencegahan kebocoran query), rotasi token JWT, dan hashing bcrypt.
10. **[`docs/TESTING.md`](docs/TESTING.md)**: **Panduan Unit & Integration Testing.** Cara menjalankan test suite, mocking time, dan validasi formula priority engine.
11. **[`docs/ERRORS.md`](docs/ERRORS.md)**: **Troubleshooting & Known Errors.** Daftar error yang pernah dihadapi dan panduan resolusinya.
12. **[`docs/STATE.md`](docs/STATE.md)**: **Status Terkini & Handover.** Catatan serah terima kondisi kode terakhir sebelum melanjutkan iterasi berikutnya.
13. **[`docs/to-do.md`](docs/to-do.md)**: **Master Task Backlog.** Roadmap pengerjaan dari Tahap 1 hingga Tahap 5.
14. **[`docs/DEPLOY.md`](docs/DEPLOY.md)**: **Panduan Deployment.** Deployment di Docker / VPS / Cloud server.

---

## 🧪 Verifikasi & Uji Kode

Pastikan seluruh linter, unit test, dan kompilasi bersih sebelum melakukan commit:
```bash
go test -v ./... && go vet ./... && go build ./...
```
