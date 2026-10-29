# Stoodify API — Panduan untuk Developer Mobile

Selamat datang! Dokumen ini berisi **semua yang dibutuhkan** untuk menghubungkan aplikasi React Native (Expo) ke backend Stoodify. Ditulis khusus untuk tim Mobile Dev: alurnya dijelaskan dari sisi aplikasi, bukan dari sisi server.

- **Base URL (lokal):** `http://localhost:8080/api`
- **Base URL (production):** diisi setelah deploy Railway/Render
- **Semua request** mengirim dan menerima `Content-Type: application/json`

> Semua contoh `curl` bisa langsung dicoba. Ganti `<TOKEN>` dengan token hasil login.

---

## 1. Alur Login (wajib dipahami dulu)

Stoodify memakai **JWT dengan 2 token**:

| Token | Umur | Dipakai untuk |
|---|---|---|
| `access_token` | 15 menit | Semua request yang butuh login (header `Authorization`) |
| `refresh_token` | 7 hari | Menukar dengan access token baru kalau access habis masa |

**Aturan penting:**
- `access_token` cepat habis (15 menit). Jangan paksa user login ulang — pakai `/auth/refresh`.
- `refresh_token` **hanya bisa dipakai sekali**. Setiap refresh, server memberi pasangan token baru dan mencabut yang lama.
- `refresh_token` yang sudah dipakai/dicabut → error 401. Kalau ini terjadi, minta user login ulang.

### 1.1 Register

```
POST /auth/register
```

```json
{
  "name": "Aliezzar",
  "email": "liezzar@gmail.com",
  "school": "SMK TELKOM SIDOARJO",
  "grade": 10,
  "major": "Teknik Komunikasi Jaringan",
  "password": "rahasia123",
  "confirm_password": "rahasia123"
}
```

Sukses → **201**:
```json
{
  "success": true,
  "message": "registrasi berhasil, sekarang harap login",
  "data": {
    "id": 55,
    "created_at": "2026-10-30T00:31:31.054221943+07:00",
    "updated_at": "2026-10-30T00:31:31.054221943+07:00",
    "name": "Aliezzar",
    "email": "liezzar@gmail.com",
    "role": "student",
    "school": "SMK TELKOM SIDOARJO",
    "grade": 10,
    "major": "Teknik Komunikasi Jaringan"
  }
}
```

| Error | Status | Kapan muncul |
|---|---|---|
| Email sudah dipakai | 409 | `errors.body = "email registered"` |
| Password ≠ konfirmasi | 422 | `errors.confirm_password` |
| Field kosong / email tidak valid | 400 | `errors.body` |

### 1.2 Login

```
POST /auth/login
```

```json
{ "email": "liezzar@gmail.com", "password": "rahasia123" }
```

Sukses → **200**:
```json
{
  "success": true,
  "message": "login berhasil",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9....",
    "expires_in": 900,
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9....",
    "token_type": "Bearer",
    "user": {
      "id": 55,
      "created_at": "2026-10-30T00:31:31.054221+07:00",
      "updated_at": "2026-10-30T00:31:31.054221+07:00",
      "name": "Aliezzar",
      "email": "liezzar@gmail.com",
      "role": "student",
      "school": "SMK TELKOM SIDOARJO",
      "grade": 10,
      "major": "Teknik Komunikasi Jaringan"
    }
  }
}
```

💡 **Catatan keamanan:** `expires_in` = 900 detik (15 menit). Pesan error untuk password salah dan email tidak terdaftar **selalu sama** ("email atau password salah") — ini disengaja, jangan dijadikan pembeda di aplikasi.

### 1.3 Kirim token di tiap request

Semua endpoint (kecuali register/login/refresh) butuh header ini:

```
Authorization: Bearer <access_token>
```

Contoh curl:
```bash
curl -X GET http://localhost:8080/api/tasks/ \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

Kalau `access_token` habis → **401** "token tidak valid atau sudah kedaluwarsa". Tangani dengan memanggil refresh (1.4).

### 1.4 Refresh token (saat access habis)

```
POST /auth/refresh
```

```json
{ "refresh_token": "eyJhbGciOiJIUzI1NiIs..." }
```

Sukses → **200**, server memberikan pasangan **baru**:
```json
{
  "success": true,
  "message": "token diperbarui",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "token_type": "Bearer",
    "expires_in": 900
  }
}
```

**Pola yang disarankan di aplikasi (interceptor axios/TanStack Query):**
1. Request dipanggil dengan `access_token`.
2. Dapat 401 → panggil `/auth/refresh` dengan `refresh_token`.
3. Sukses → simpan token baru, ulangi request yang gagal.
4. Refresh gagal (401) → arahkan ke layar login.

### 1.5 Logout

```
POST /auth/logout
```

Tanpa body: mencabut **semua** refresh token user (aman: user keluar dari semua perangkat).
Dengan body: mencabut satu perangkat saja.
```json
{ "refresh_token": "eyJhbGciOiJIUzI1NiIs..." }
```

Sukses → **200**. Setelah logout, refresh token tidak bisa dipakai lagi.

### 1.6 Cek user saat ini

```
GET /auth/me
```
Sukses → **200** dengan data user (field `password_hash` tidak pernah dikirim).

---

## 2. Format Response (semua endpoint)

Sukses:
```json
{ "success": true, "message": "pesan", "data": { }, "meta": { } }
```

Error:
```json
{ "success": false, "message": "pesan", "errors": { "field": "alasan" } }
```

**Kode status yang umum:**

| Kode | Arti |
|---|---|
| 200 | Sukses (GET/PATCH) |
| 201 | Berhasil dibuat (POST) |
| 204 | Berhasil dihapus (DELETE, tanpa body) |
| 400 | Body tidak valid / salah format |
| 401 | Token hilang / salah / habis |
| 404 | Data tidak ditemukan (atau bukan milik user) |
| 409 | Konflik (email sudah dipakai, jadwal bentrok) |
| 422 | Data valid format tapi tidak masuk akal (mis. password beda, jam berakhir sebelum mulai) |
| 500 | Error server (bukan salah client) |

> ⚠️ **PENTING untuk keamanan:** kalau user mencoba akses data orang lain (mis. task id milik user lain), server balas **404**, bukan 403 — supaya tidak bisa ditebak apakah id itu ada. Di aplikasi, perlakukan 404 sebagai "tidak ada / tidak diizinkan".

**Pagination** (endpoint GET yang bisa panjang): kirim `?page=1&per_page=20`. Balasan `meta`:
```json
"meta": { "page": 1, "perPage": 20, "total": 42, "totalPages": 3 }
```

---

## 3. Profil & Preferensi

### Ringkasan

| Method | Path | Fungsi |
|---|---|---|
| GET | `/users/me` | Ambil profil sendiri |
| PATCH | `/users/me` | Ubah nama / email |
| GET | `/users/me/preferences` | Ambil preferensi belajar |
| PATCH | `/users/me/preferences` | Simpan preferensi belajar |

### Contoh: atur preferensi waktu belajar

```
PATCH /users/me/preferences
```
```json
{
  "start_time": "15:00",
  "end_time": "21:00",
  "session_duration": 60,
  "active_days": "1,2,3,4,5"
}
```

| Field | Format | Catatan |
|---|---|---|
| `start_time` / `end_time` | `HH:MM` | Boleh dikirim sebagian (PATCH parsial) |
| `session_duration` | angka menit | 15–600. Dipakai AI scheduler sebagai durasi sesi |
| `active_days` | string | `1,2,3,4,5` = Senin–Jumat (1=Senin, 7=Minggu) |

🌱 **Sebelum user mulai belajar**, preferensi ini penting — AI scheduler sangat bergantung padanya.

---

## 4. Mata Pelajaran (Subjects)

### Ringkasan

| Method | Path | Fungsi |
|---|---|---|
| GET | `/subjects/?search=&page=&per_page=` | Daftar mapel |
| POST | `/subjects/` | Tambah mapel |
| GET | `/subjects/{id}` | Detail mapel |
| PATCH | `/subjects/{id}` | Ubah mapel |
| DELETE | `/subjects/{id}` | Hapus mapel |

### Contoh: tambah mapel

```
POST /subjects/
```
```json
{ "name": "Matematika", "color": "#FF5252", "difficulty": 4 }
```

Sukses → **201**:
```json
{
  "success": true,
  "message": "mata pelajaran dibuat",
  "data": { "id": 5, "user_id": 1, "name": "Matematika", "color": "#FF5252", "difficulty": 4 }
}
```

| Field | Wajib | Catatan |
|---|---|---|
| `name` | ✅ | 1–100 karakter |
| `color` | | hex, mis. `#FF5252` |
| `difficulty` | | 1 (mudah) – 5 (sangat sulit). Dipakai priority engine |

---

## 5. Jadwal Sekolah (Schedules) + Deteksi Bentrok

### Ringkasan

| Method | Path | Fungsi |
|---|---|---|
| GET | `/schedules/` | Semua jadwal |
| POST | `/schedules/` | Tambah jadwal |
| GET | `/schedules/today` | Jadwal hari ini |
| GET | `/schedules/week` | Jadwal sepekan (urut hari Senin→Minggu) |
| PATCH | `/schedules/{id}` | Ubah jadwal |
| DELETE | `/schedules/{id}` | Hapus jadwal |

### Contoh: tambah jadwal

```
POST /schedules/
```
```json
{
  "subject_id": 5,
  "day_of_week": "Monday",
  "start_time": "07:00",
  "end_time": "08:30"
}
```

Sukses → **201**. Data balasan menyertakan objek `subject` (nama, warna) — tidak perlu fetch terpisah.

### Fitur: deteksi bentrok otomatis

Server menolak jadwal yang **tumpang-tindih** di hari yang sama:

```
POST /schedules/   (08:00–09:00, padahal sudah ada 07:00–08:30)
```
→ **409**:
```json
{
  "success": false,
  "message": "jadwal bentrok dengan jadwal lain di hari yang sama",
  "errors": { "conflict_with": "07:00-08:30" }
}
```

💡 **Tips UI:** pakai `errors.conflict_with` untuk menampilkan "Bentrok dengan jadwal 07:00–08:30".

| Validasi lain | Status |
|---|---|
| `end_time <= start_time` | 422 |
| `day_of_week` selain `Monday`–`Sunday` | 400 |
| `subject_id` bukan milik user | 404 |

---

## 6. Tugas (Tasks) — jantung aplikasi

### Ringkasan

| Method | Path | Fungsi |
|---|---|---|
| GET | `/tasks/?status=&subject_id=&search=&sort=priority&page=&per_page=` | Daftar + filter |
| POST | `/tasks/` | Tambah tugas |
| GET | `/tasks/{id}` | Detail (ikut subtasks & subject) |
| PATCH | `/tasks/{id}` | Ubah tugas |
| DELETE | `/tasks/{id}` | Hapus tugas |
| PATCH | `/tasks/{id}/status` | Ubah status |
| PATCH | `/tasks/{id}/progress` | Ubah progres |
| GET | `/tasks/upcoming` | Deadline 7 hari ke depan |
| GET | `/tasks/overdue` | Sudah lewat deadline |

### Contoh: tambah tugas

```
POST /tasks/
```
```json
{
  "subject_id": 5,
  "title": "PR Integral",
  "description": "Nomor 1–10 halaman 45",
  "type": "exam",
  "deadline": "2026-10-10T23:59:00+07:00",
  "difficulty": 5,
  "estimated_duration": 90,
  "notes": "Kerjakan pakai buku catatan"
}
```

Sukses → **201**. Perhatikan: response punya **`priority_score`** yang dihitung otomatis server:
```json
{
  "success": true,
  "message": "tugas dibuat",
  "data": {
    "id": 12, "user_id": 1, "subject_id": 5,
    "title": "PR Integral", "type": "exam",
    "deadline": "2026-10-10T23:59:00+07:00",
    "priority_score": 87.4,
    "status": "pending", "progress": 0
  }
}
```

### Penjelasan field

| Field | Wajib | Nilai / Format |
|---|---|---|
| `subject_id` | ✅ | id mapel milik user |
| `title` | ✅ | 1–200 karakter |
| `deadline` | ✅ | RFC3339 dengan zona waktu: `2026-10-10T23:59:00+07:00` |
| `difficulty` | ✅ | 1–5 |
| `type` | | `exam` (ujian), `project`, `assignment` (tugas biasa), `reading` (baca materi) |
| `estimated_duration` | | menit (default 60). Dipakai priority & AI scheduler |
| `description`, `notes` | | teks bebas |
| `priority_score` | — | **JANGAN dikirim** client; dihitung server (0–100) |
| `status` | — | `pending` → `in_progress` → `done` / `late`. Diubah lewat endpoint status, bukan langsung |

### Mengubah status & progres

```
PATCH /tasks/{id}/status     { "status": "in_progress" }
PATCH /tasks/{id}/progress   { "progress": 50 }
```

- `progress` 0–100. Mencapai **100 otomatis menandai `done`**.
- Set `status: "done"` memaksa progres jadi 100.
- Status valid: `pending`, `in_progress`, `done`, `late`. `late` muncul otomatis oleh scheduler saat deadline lewat.

### Lupa ngerjain? Server otomatis tandai "Terlambat"

Tiap menit, server mengecek: tugas `pending`/`in_progress` yang deadlinenya sudah lewat → status berubah jadi `late`. Aplikasi cukup refetch daftar tugas (mis. saat screen fokus) untuk dapat status terbaru.

### Filters di GET /tasks/

| Query | Contoh | Hasil |
|---|---|---|
| `status` | `?status=pending` | Hanya pending |
| `subject_id` | `?subject_id=5` | Hanya mapel 5 |
| `search` | `?search=integral` | Cari di judul (case-insensitive) |
| `sort=priority` | `?sort=priority` | Urut skor prioritas tertinggi dulu (default: deadline terdekat) |
| `page` / `per_page` | `?page=2&per_page=10` | Pagination |

---

## 7. Subtugas (Subtasks)

Memecah tugas besar jadi langkah kecil.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/tasks/{id}/subtasks/` | Daftar subtugas sebuah tugas |
| POST | `/tasks/{id}/subtasks/` | Tambah subtugas |
| PATCH | `/subtasks/{id}` | Ubah judul / centang selesai |
| DELETE | `/subtasks/{id}` | Hapus subtugas |

```json
POST /tasks/12/subtasks/    { "title": "Kerjakan nomor 1–5" }
PATCH /subtasks/3           { "is_completed": true }
```

---

## 8. Aktivitas Rutin (Activities)

Kegiatan berulang (ekskul, les, ibadah) yang **memakan waktu** — AI scheduler memperhitungkannya sebagai waktu yang tidak bisa dipakai belajar.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/activities/` | Daftar aktivitas |
| POST | `/activities/` | Tambah |
| PATCH | `/activities/{id}` | Ubah |
| DELETE | `/activities/{id}` | Hapus |

```json
POST /activities/
{ "title": "Latihan Basket", "start_time": "16:00", "end_time": "17:30", "recurrence": "weekly" }
```

`recurrence`: `daily`, `weekly` (default), `monthly`, `none`.

---

## 9. AI Bridge — Rekomendasi & Prediksi

Bagian ini menghubungkan aplikasi ke AI Stoodify. **Client tidak pernah memanggil AI langsung** — semua lewat backend (API key aman di server).

### 9.1 Generate rekomendasi sesi belajar

```
POST /ai/schedule-recommendations/generate
```
Body opsional: `{ "date": "2026-10-10" }` (default: hari ini).

Server mengambil tugas aktif user, meminta AI menyusun jadwal belajar di slot kosong, lalu **langsung menyimpan hasilnya sebagai sesi belajar** (`study_sessions`).

Sukses → **201**, `data` berisi array sesi:
```json
{
  "success": true,
  "message": "rekomendasi sesi belajar dibuat (fallback rule-based)",
  "data": [
    {
      "id": 30,
      "task_id": 12,
      "session_date": "2026-10-10",
      "start_time": "19:00",
      "duration": 60,
      "target_progress": 30,
      "status": "planned",
      "ai_generated": true,
      "reason": "Skor prioritas 87, deadline 10 Okt 2026, progres baru 0%"
    }
  ]
}
```

> ℹ️ `message` menyebut sumber: `(ai)` kalau AI Specialist merespons, `(fallback rule-based)` kalau AI mati. Bisa dipakai untuk menampilkan label yang lebih ramah.

### 9.2 Accept / reject rekomendasi

```
POST /ai/schedule-recommendations/{id}/accept
POST /ai/schedule-recommendations/{id}/reject
```
- **accept** → sesi tetap di kalender (status `planned`)
- **reject** → sesi ditandai `skipped`
- Daftar semua rekomendasi: `GET /ai/schedule-recommendations`

### 9.3 Prediksi materi (AI Spec 2)

```
POST /ai/material-predictions/generate
```
```json
{ "subject_id": 5 }
```
Server membaca riwayat tugas mapel itu, meminta AI memprediksi materi berikutnya, lalu menyimpan. Sukses → **201**.

Ambil riwayat prediksi sebuah mapel: `GET /ai/material-predictions/{subject_id}`
Kirim umpan balik (membantu AI makin pintar): `POST /ai/material-predictions/{id}/feedback` dengan `{ "feedback": "accurate" }` atau `"inaccurate"`.

---

## 10. Notifikasi

Dibuat otomatis oleh scheduler (H-1 & H-0 deadline). Aplikasi cukup menampilkan.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/notifications/?unread=true&page=&per_page=` | Daftar notifikasi |
| PATCH | `/notifications/{id}/read` | Tandai satu dibaca |
| PATCH | `/notifications/read-all` | Tandai semua dibaca |

Contoh item notifikasi:
```json
{
  "id": 3,
  "user_id": 1,
  "task_id": 12,
  "title": "Deadline besok",
  "message": "Tugas \"PR Integral\" harus selesai 10 Okt 2026 23:59",
  "type": "reminder",
  "scheduled_at": "2026-10-09T19:00:00+07:00",
  "is_read": false
}
```

`type`: `reminder` (pengingat), `warning`, `late`.

---

## 11. Sesi Belajar (Study Sessions)

Sesi belajar yang muncul di kalender — hasil rekomendasi AI atau dibuat manual.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/study-sessions/?status=&page=&per_page=` | Daftar sesi |
| GET | `/study-sessions/today` | Sesi hari ini |
| PATCH | `/study-sessions/{id}` | Reschedule manual / ubah status |
| DELETE | `/study-sessions/{id}` | Hapus sesi |

Contoh reschedule manual (user mengubah waktu sesi):
```json
PATCH /study-sessions/30
{ "session_date": "2026-10-11", "start_time": "20:00", "duration": 45 }
```

Mengubah sesi buatan AI secara manual otomatis menandai `ai_generated = false` (sesi jadi "milik user").

---

## 12. Statistik

Untuk dashboard siswa.

| Method | Path | Isi data |
|---|---|---|
| GET | `/statistics/overview` | Total tugas, selesai, terlambat, % tepat waktu, total jam belajar, jumlah mapel |
| GET | `/statistics/weekly` | Sama, untuk 7 hari terakhir |
| GET | `/statistics/monthly` | Sama, untuk 30 hari terakhir |
| GET | `/statistics/subjects` | Rincian per mapel (total/selesai/terlambat + rata-rata skor) |

`overview` contoh:
```json
{
  "total_tasks": 12,
  "done_tasks": 7,
  "late_tasks": 2,
  "pending_tasks": 3,
  "on_time_rate": 77.78,
  "total_study_minutes": 540,
  "total_subjects": 4
}
```

---

## 13. Alur Nyata di Aplikasi (cheat sheet)

### Onboarding (register → atur preferensi)

```
1. POST /auth/register              → 201
2. POST /auth/login                 → simpan access + refresh token
3. PATCH /users/me/preferences      → waktu belajar & durasi sesi
4. POST /subjects/                  → tambah mapel (loop)
5. POST /schedules/                 → tambah jadwal (loop)
```

### Dashboard harian

```
GET /tasks/upcoming                 → tugas 7 hari ke depan
GET /tasks/overdue                  → tugas telat
GET /study-sessions/today           → sesi hari ini
GET /notifications/?unread=true     → pengingat
GET /statistics/overview            → poin statistik
```

### Tambah tugas → AI kasih jadwal

```
1. POST /tasks/                     → simpan id tugas, priority_score otomatis
2. POST /ai/schedule-recommendations/generate  → sesi AI dibuat
3. GET /ai/schedule-recommendations → tampilkan daftar + alasan (field "reason")
4. POST /ai/schedule-recommendations/{id}/accept  → masuk kalender
```

### Sesi dilewati → reschedule

```
POST /ai/reschedule                 → semua sesi planned disusun ulang ke slot kosong hari ini
```

---

## 14. Lampiran: Sumber AI

| Variabel | Fungsi |
|---|---|
| `AI_SCHEDULER_URL` | Endpoint AI Spec 1 (penyusun jadwal) |
| `AI_PREDICTOR_URL` | Endpoint AI Spec 2 (prediksi materi) |
| `AI_API_KEY` | Api key bearer untuk kedua endpoint |
| `AI_FALLBACK_ENABLED` | `false` = matikan fallback rule-based |

AI memberi rekomendasi, **siswa tetap yang memutuskan** (bisa tolak/ubah). Setiap output AI wajib menyertakan alasan (`reason`) yang mudah dipahami siswa — tampilkan field itu apa adanya.