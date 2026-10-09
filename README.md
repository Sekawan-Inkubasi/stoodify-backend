# Stoodify Backend

REST API Stoodify — asisten belajar berbasis AI untuk siswa SMP/SMA/SMK. Golang + Gin + GORM + Supabase PostgreSQL.

## Menjalankan

```bash
cp .env.example .env      # lalu isi POSTGRES_URL & JWT_SECRET
go mod download
go run .                  # http://localhost:8080
```

Tabel + index dibuat otomatis saat boot (`AutoMigrate`), tidak perlu migrasi manual.

## Isi Proyek

```
main.go            # boot: DB -> scheduler -> router -> listen (graceful shutdown)
config/            # koneksi GORM ke Supabase
routes/            # registrasi seluruh route + middleware
controllers/       # HTTP layer (bind/validasi/respond, query via ctrl.DB)
middlewares/       # RequireAuth (JWT), RequireRole (RBAC)
migrations/        # model GORM + AutoMigrate + index kinerja
workers/           # scheduler: auto-"late" + notifikasi H-1/H-0
internal/token/    # JWT access 15m + refresh 7d (rotasi, hash di DB)
internal/response/ # envelope sukses/error
internal/priority/ # Priority Score Engine (formula PRD)
internal/params/   # parsing id & pagination
docs/API.md        # dokumentasi semua endpoint
docs/DEPLOY.md     # deploy Railway/Render + Supabase
```

## Dokumentasi

- Endpoint lengkap, request/response, kode error: [`docs/API.md`](docs/API.md)
- Deploy production: [`docs/DEPLOY.md`](docs/DEPLOY.md)

## Ringkas

- **Auth** — JWT HS256, access 15 menit, refresh 7 hari dengan rotasi + pencabutan (logout). Password bcrypt cost 14.
- **RBAC** — `RequireRole("admin", ...)` siap dipakai di route mana pun.
- **Ownership** — setiap query difilter `user_id` dari token; milik user lain → 404.
- **Priority Engine** — skor 0–100 dari deadline urgency, difficulty, duration, importance, late-risk, dikurangi progres.
- **Scheduler** — ticker 1 menit: tugas lewat deadline → `late`, notifikasi H-1 & H-0 (idempotent per hari).
- **AI Bridge** — kirim payload ke AI Specialist, validasi output, simpan ke DB; fallback rule-based bila AI gagal (>15 detik).

## Verifikasi

```bash
go build ./... && go vet ./... && gofmt -l .   # semua harus bersih
```
