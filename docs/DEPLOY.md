# Deployment

Backend Go ini butuh satu proses (HTTP + scheduler goroutine) dan satu PostgreSQL (Supabase).

## Wajib diset di server

| Env | Nilai |
|---|---|
| `POSTGRES_URL` | DSN Supabase. Kalau lewat pooler: `postgresql://postgres.<ref>:<password>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require`. Direct: port `5432`. Bisa juga `DATABASE_URL` / `DATABASE_URL_UNPOOLED` (Neon). |
| `JWT_SECRET` | `openssl rand -hex 32` |
| `PORT` | diisi otomatis oleh Railway/Render — jangan hardcode |
| `TRUSTED_PROXIES` | IP/CIDR reverse proxy, mis. `10.0.0.0/8`. Kosong = percaya semua `X-Forwarded-For` (rate limiter jadi bisa dibohongi) |
| `AI_SCHEDULER_URL` | **wajib** untuk `serve` (kosong = server menolak start) |
| `AI_PREDICTOR_URL` | **wajib** untuk `serve` (kosong = server menolak start) |
| `AI_API_KEY` | opsional; dikirim sebagai Bearer ke kedua endpoint |
| `AI_TIMEOUT_SECONDS` | opsional, default `15`. Lewat batas ini klien dapat `502` |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` | opsional, untuk email reset password. Kosong = `NoopMailer` (email tidak dikirim, token tetap dibuat) |
| `PASSWORD_RESET_URL` | base URL halaman reset di aplikasi, dipakai di body email |
| `EMAIL_VERIFICATION_URL` | URL halaman **jembatan** verifikasi di backend, mis. `https://api.domainmu/verify-email` — dipakai di body email |
| `EMAIL_VERIFICATION_DEEPLINK` | Skema deep link aplikasi, default `stoodify://verify-email`. Harus sama dengan konfigurasi app |
| `SMTP_HOST`/`SMTP_PORT`/`SMTP_USERNAME`/`SMTP_PASSWORD`/`SMTP_FROM` | Wajib untuk production: tanpa `SMTP_HOST`, `NoopMailer` aktif dan pendaftar tidak pernah menerima link verifikasi (tidak bisa login) |

AutoMigrate + index dijalankan saat boot, jadi deploy pertama langsung membuat tabel.

> ⚠️ **Avatar disimpan di disk lokal** (`uploads/avatars`, disajikan lewat `GET /uploads/...`). Filesystem Railway/Render bersifat *ephemeral*: avatar akan hilang setiap redeploy/restart dan tidak akan sinkron kalau instance lebih dari satu. Untuk produksi, pindahkan ke object storage dulu (helper `config.InitStorage` sudah membaca `AWS_*` tapi belum dipakai handler mana pun).

## Railway

1. New Project → Deploy from GitHub repo ini, root directory `backend`/`stoodify-backend`.
2. Railway mendeteksi `Dockerfile` otomatis.
3. Set env di tab Variables.
4. Deploy. Health check: pakai path `/health` (membalas `200 {"status":"healthy","service":"stoodify-backend"}` tanpa auth).
5. Endpoint `/api/*` butuh token; jangan pakai endpoint ber-auth sebagai health check (akan selalu 401).

## Variabel Environment Penting

| Variabel | Nilai produksi | Efek |
| --- | --- | --- |
| `APP_ENV` | `production` | Menyembunyikan detail error internal (nama struct/tag validator) dari client; `errors` tetap berisi pesan per field |
| `TRUSTED_PROXIES` | IP/CIDR reverse proxy | Header XFF hanya dipercaya dari proxy ini (rate limit per IP benar) |
| `AI_SCHEDULER_URL`, `AI_PREDICTOR_URL` | URL service AI | Wajib; `serve` gagal start bila kosong |

`docker-compose.yml` sudah menetapkan `APP_ENV=production` untuk container produksi.

## Render

- Environment: **Docker**
- Dockerfile Path: `Dockerfile`
- Health Check Path: `/health`
- Set env yang sama di tab Environment.

## Catatan Supabase

- Free tier tidur setelah idle; request pertama bisa lambat (~1–3 detik). `POSTGRES_URL` wajib `sslmode=require`.
- Gunakan connection **pooler** untuk production agar tidak kehabisan slot koneksi.
- `alpine` runtime sudah memuat `tzdata`; jadwal & deadline memakai zona container (`TZ=Asia/Jakarta`).

## Cek setelah deploy

```bash
curl -s -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' \
  -d '{"name":"Smoke","email":"smoke@example.com","password":"rahasia123","confirm_password":"rahasia123"}'
curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"smoke@example.com","password":"rahasia123"}'
```
Login harus membalas `access_token` dan `refresh_token`.
