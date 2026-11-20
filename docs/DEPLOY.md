# Deployment

Backend Go ini butuh satu proses (HTTP + scheduler goroutine) dan satu PostgreSQL (Supabase/Neon/pg biasa).

## Wajib diset di server

| Env | Nilai |
|---|---|
| `POSTGRES_URL` | DSN PostgreSQL (Supabase/Neon). Kalau lewat pooler Supabase: `postgresql://postgres.<ref>:<password>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require`. Direct: port `5432`. Bisa juga `DATABASE_URL` / `DATABASE_URL_UNPOOLED` (Neon). |
| `JWT_SECRET` | `openssl rand -hex 32` |
| `PORT` | diisi otomatis oleh Railway/Render — jangan hardcode |
| `TRUSTED_PROXIES` | IP/CIDR reverse proxy, mis. `10.0.0.0/8`. Kosong = percaya semua `X-Forwarded-For` (rate limiter jadi bisa dibohongi) |
| `AI_SCHEDULER_URL` | **wajib** untuk `serve` (kosong = server menolak start) |
| `AI_PREDICTOR_URL` | **wajib** untuk `serve` (kosong = server menolak start) |
| `AI_API_KEY` | opsional; dikirim sebagai Bearer ke kedua endpoint |
| `AI_TIMEOUT_SECONDS` | opsional, default `15`. Lewat batas ini klien dapat `502` |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` | **Wajib** untuk production: dipakai reset password dan verifikasi email. Kosong = `NoopMailer` (tidak ada email terkirim, tanpa error) sehingga pendaftar baru tidak bisa login |
| `EMAIL_VERIFICATION_URL` | URL halaman jembatan verifikasi di backend, mis. `https://api.domainmu/verify-email` — dipakai di body email |
| `EMAIL_VERIFICATION_DEEPLINK` | Skema deep link aplikasi, default `stoodify://verify-email`. Harus sama dengan konfigurasi app |
| `PASSWORD_RESET_URL` | base URL halaman reset di aplikasi, dipakai di body email |
| `APP_ENV` | `production` (sudah di `docker-compose.yml`). Menyembunyikan detail error internal dari client |
| `JWT_ACCESS_TTL` / `JWT_REFRESH_TTL` | opsional; kosong = 15m / 168h |
| `TZ` | zona waktu untuk engine AI (default `Asia/Jakarta`) |

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

## CLI Operasional (untuk DevOps)

Binary `stoodify` sudah membawa perintah operasional, jadi tidak perlu skrip tambahan di server.

| Perintah | Fungsi |
| --- | --- |
| `stoodify doctor` | Periksa env wajib, konektivitas AI, dan database. Exit 1 bila ada `[FAIL]` — pakai sebagai gate sebelum deploy. `--json` untuk monitoring. |
| `stoodify version` | Tampilkan versi/commit/build date yang **sedang berjalan** (diisi lewat `-ldflags` saat build Docker). |
| `stoodify db:backup` | `pg_dump` + gzip ke `./backups/stoodify-<waktu>.sql.gz`, rotasi otomatis (`--keep`, default 7). |
| `stoodify db:restore` | Pulihkan dari `--file=<path>` atau `--latest`. Di production wajib `--force`. |
| `stoodify rollback` | Pulihkan database ke backup terakhir (`--step`), lalu AutoMigrate agar skema mutakhir. `--list` untuk melihat titik pemulihan. |
| `stoodify migrate` / `migrate:fresh` / `seed` | Seperti sebelumnya. |

### Alur deploy yang disarankan

```bash
stoodify doctor              # 1. gate: jangan lanjut kalau ada [FAIL]
stoodify db:backup           # 2. snapshot sebelum perubahan skema
stoodify migrate             # 3. terapkan migrasi
# ... deploy kode lewat CI (docker compose up -d --build app) ...
```

### Kalau deploy bermasalah

```bash
stoodify rollback --list     # lihat titik pemulihan
stoodify rollback --step=1   # pulihkan DATA terakhir
git reset --hard <sha-lama>  # pulihkan KODE
docker compose up -d --build app
```

**Batasan yang perlu dipahami:** `rollback` hanya mengembalikan **data**, bukan binary. Rollback kode tetap lewat git/CI karena binary tidak bisa menurunkan versi dirinya sendiri. Kedua langkah harus dilakukan bersamaan kalau migrasi ikut berubah.

### Catatan backup

- File backup berisi **seluruh isi database** (termasuk hash password & email) — perlakukan sebagai rahasia. Direktori `./backups/` sudah masuk `.gitignore`.
- Backup disimpan **lokal** di server; untuk produksi sebaiknya disalin ke object storage karena filesystem container bersifat ephemeral.
- `pg_dump`/`psql` sudah tersedia di image (`postgresql16-client`). Kalau menjalankan binary di luar Docker, pasang dulu `postgresql-client`.

