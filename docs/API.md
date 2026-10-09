# Stoodify API

Base URL lokal: `http://localhost:8080/api`
Semua endpoint kecuali register/login/refresh butuh header `Authorization: Bearer <access_token>`.

## Envelope Response

Sukses:
```json
{ "success": true, "message": "...", "data": { }, "meta": { "page": 1, "perPage": 20, "total": 42, "totalPages": 3 } }
```
Error:
```json
{ "success": false, "message": "...", "errors": { "field": "alasan" } }
```

## Auth

| Method | Path | Auth | Request | Sukses |
|---|---|---|---|---|
| POST | `/auth/register` | — | `{name, email, password, confirm_password}` | 201 |
| POST | `/auth/login` | — | `{email, password}` | 200 |
| POST | `/auth/refresh` | — | `{refresh_token}` | 200 |
| POST | `/auth/logout` | Bearer | `{refresh_token}` (opsional) | 200 |
| GET | `/auth/me` | Bearer | — | 200 |

`POST /auth/login` sukses:
```json
{
  "success": true,
  "message": "login berhasil",
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<jwt>",
    "token_type": "Bearer",
    "expires_in": 900,
    "user": { "id": 1, "name": "Budi", "email": "budi@mail.com", "role": "student" }
  }
}
```

Aturan token: access 15 menit, refresh 7 hari. `POST /auth/refresh` merotasi refresh token — token lama langsung dicabut, dipakai ulang akan 401. Logout mencabut semua refresh token aktif user (atau satu token bila `refresh_token` dikirim).

Kode error auth:
| Kasus | Status |
|---|---|
| register email sudah dipakai | 409 |
| register password != konfirmasi | 422 |
| login email/password salah | 401 (pesan sama untuk email tidak ada) |
| refresh token dicabut/kedaluwarsa | 401 |
| endpoint protected tanpa token valid | 401 |

## Users & Preferences

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/users/me` | — | 200 |
| PATCH | `/users/me` | `{name?, email?}` | 200 |
| GET | `/users/me/preferences` | — | 200 |
| PATCH | `/users/me/preferences` | `{start_time?, end_time?, session_duration?, active_days?}` | 200 |

`start_time`/`end_time` format `HH:MM`, `session_duration` menit (15–600), `active_days` contoh `"1,2,3,4,5"`.

## Subjects

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/subjects/?search=&page=&per_page=` | — | 200 + meta |
| POST | `/subjects/` | `{name, color?, difficulty?}` | 201 |
| GET | `/subjects/{id}` | — | 200 |
| PATCH | `/subjects/{id}` | `{name, color?, difficulty?}` | 200 |
| DELETE | `/subjects/{id}` | — | 204 |

`difficulty` 1–5. Semua query difilter `user_id` dari token; milik user lain → 404.

## Schedules

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/schedules/` | — | 200 |
| POST | `/schedules/` | `{subject_id, day_of_week, start_time, end_time}` | 201 |
| GET | `/schedules/today` | — | 200 |
| GET | `/schedules/week` | — | 200 |
| PATCH | `/schedules/{id}` | sama seperti POST | 200 |
| DELETE | `/schedules/{id}` | — | 204 |

`day_of_week`: `Monday`..`Sunday`. Bentrok waktu (overlap di hari sama) → 409 dengan `errors.conflict_with`. `end_time <= start_time` → 422.

## Tasks

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/tasks/?status=&subject_id=&search=&sort=priority&page=&per_page=` | — | 200 + meta |
| POST | `/tasks/` | `{subject_id, title, description?, type?, deadline, difficulty, estimated_duration?, notes?}` | 201 |
| GET | `/tasks/{id}` | — | 200 |
| PATCH | `/tasks/{id}` | sama seperti POST | 200 |
| DELETE | `/tasks/{id}` | — | 204 |
| PATCH | `/tasks/{id}/status` | `{status}` | 200 |
| PATCH | `/tasks/{id}/progress` | `{progress}` | 200 |
| GET | `/tasks/upcoming` | — | 200 |
| GET | `/tasks/overdue` | — | 200 |

`type`: `exam` \| `project` \| `assignment` \| `reading`.
`status`: `pending` \| `in_progress` \| `done` \| `late`.
`deadline` RFC3339 (`2026-10-20T23:59:00+07:00`).
`priority_score` dihitung otomatis (0–100) memakai formula PRD: urgency 40%, difficulty 20%, duration 15%, importance 15%, late-risk 10%, dikurangi bobot progres.
`progress` 100 otomatis menandai `done`; `status=done` memaksa progres 100.

## Subtasks

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/tasks/{id}/subtasks/` | — | 200 |
| POST | `/tasks/{id}/subtasks/` | `{title}` | 201 |
| PATCH | `/subtasks/{id}` | `{title?, is_completed?}` | 200 |
| DELETE | `/subtasks/{id}` | — | 204 |

## Activities

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/activities/` | — | 200 |
| POST | `/activities/` | `{title, start_time, end_time, recurrence?}` | 201 |
| PATCH | `/activities/{id}` | sama seperti POST | 200 |
| DELETE | `/activities/{id}` | — | 204 |

`recurrence`: `daily` \| `weekly` \| `monthly` \| `none` (default `weekly`).

## AI Bridge

| Method | Path | Request | Sukses |
|---|---|---|---|
| POST | `/ai/schedule-recommendations/generate` | `{date?}` (`YYYY-MM-DD`, default hari ini) | 201 |
| GET | `/ai/schedule-recommendations` | — | 200 |
| POST | `/ai/schedule-recommendations/{id}/accept` | — | 200 |
| POST | `/ai/schedule-recommendations/{id}/reject` | — | 200 |
| POST | `/ai/reschedule` | — | 200 |
| POST | `/ai/material-predictions/generate` | `{subject_id}` | 201 |
| GET | `/ai/material-predictions/{subject_id}` | — | 200 |
| POST | `/ai/material-predictions/{id}/feedback` | `{feedback}` (`accurate`\|`inaccurate`) | 201 |

Alur: backend mengirim payload ke AI Specialist (`AI_SCHEDULER_URL` / `AI_PREDICTOR_URL`, timeout 15 detik). Kalau AI gagal/timeout dan `AI_FALLBACK_ENABLED != "false"`, backend memakai fallback rule-based (priority engine + time-slot finder). Hasil selalu divalidasi (task harus milik user) lalu disimpan ke DB.

## Notifications

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/notifications/?unread=true&page=&per_page=` | — | 200 + meta |
| PATCH | `/notifications/{id}/read` | — | 200 |
| PATCH | `/notifications/read-all` | — | 200 |

## Study Sessions

| Method | Path | Request | Sukses |
|---|---|---|---|
| GET | `/study-sessions/?status=&page=&per_page=` | — | 200 + meta |
| GET | `/study-sessions/today` | — | 200 |
| PATCH | `/study-sessions/{id}` | `{session_date?, start_time?, duration?, status?}` | 200 |
| DELETE | `/study-sessions/{id}` | — | 204 |

## Statistics

| Method | Path | Sukses |
|---|---|---|
| GET | `/statistics/overview` | 200 |
| GET | `/statistics/weekly` | 200 |
| GET | `/statistics/monthly` | 200 |
| GET | `/statistics/subjects` | 200 |

## Scheduler (background)

Ticker tiap 1 menit:
1. Tugas `pending`/`in_progress` yang lewat deadline → status `late`.
2. Notifikasi `Deadline hari ini` (H-0) dan `Deadline besok` (H-1) untuk tugas yang belum selesai; satu notifikasi per tugas per jenis per hari.

## Environment

| Variabel | Wajib | Keterangan |
|---|---|---|
| `POSTGRES_URL` | ya | DSN Supabase, contoh `postgresql://user:pass@host:5432/postgres?sslmode=require` |
| `JWT_SECRET` | ya | ≥32 karakter acak (`openssl rand -hex 32`) |
| `PORT` | tidak | default `8080` |
| `AI_SCHEDULER_URL` | tidak | endpoint AI Spec 1; kosong = langsung fallback |
| `AI_PREDICTOR_URL` | tidak | endpoint AI Spec 2; kosong = langsung fallback |
| `AI_API_KEY` | tidak | Bearer token ke layanan AI |
| `AI_FALLBACK_ENABLED` | tidak | `false` untuk mematikan fallback rule-based |
