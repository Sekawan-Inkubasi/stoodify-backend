# Deployment

Backend Go ini butuh satu proses (HTTP + scheduler goroutine) dan satu PostgreSQL (Supabase).

## Wajib diset di server

| Env | Nilai |
|---|---|
| `POSTGRES_URL` | DSN Supabase. Kalau lewat pooler: `postgresql://postgres.<ref>:<pass>@aws-0-<region>.pooler.supabase.com:6543/postgres?sslmode=require`. Direct: port `5432`. |
| `JWT_SECRET` | `openssl rand -hex 32` |
| `PORT` | diisi otomatis oleh Railway/Render — jangan hardcode |
| `AI_SCHEDULER_URL`, `AI_PREDICTOR_URL`, `AI_API_KEY` | opsional; kosong = fallback rule-based |
| `AI_FALLBACK_ENABLED` | `false` untuk mematikan fallback |

AutoMigrate + index dijalankan saat boot, jadi deploy pertama langsung membuat tabel.

## Railway

1. New Project → Deploy from GitHub repo ini, root directory `backend`/`stoodify-backend`.
2. Railway mendeteksi `Dockerfile` otomatis.
3. Set env di tab Variables.
4. Deploy. Health check: `GET /api/auth/login` (POST) atau route apa pun yang ada — Railway hanya butuh port terbuka. Kalau ingin health check khusus, set path ke `/api/statistics/overview` **tanpa** auth akan 401; lebih aman pakai `GET /api/ngawur` yang membalas 404 (server hidup).

## Render

- Environment: **Docker**
- Dockerfile Path: `Dockerfile`
- Health Check Path: `/api/ngawur` (membalas 404 = server hidup; render hanya butuh respons HTTP)
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
