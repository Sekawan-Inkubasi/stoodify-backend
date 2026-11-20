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
| --- | --- | --- |
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

Semua field **wajib**: `name`, `email`, `school`, `grade` (7–13), `major`, `password`, `confirm_password`. Backend adalah acuan kontrak — form daftar di aplikasi wajib menyediakan `school`, `grade`, dan `major`.

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
| --- | --- | --- |
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

### 1.7 Login / Daftar dengan Google

```
POST /auth/google
{ "id_token": "<ID token dari Google Sign-In SDK>" }
```

Backend **memverifikasi sendiri** token itu (signature, issuer, `aud` = `GOOGLE_CLIENT_ID`, masa berlaku, `email_verified`). Aplikasi tidak perlu mengirim email/nama terpisah — semuanya diambil dari token.

Alur yang terjadi di server:

1. Cari user berdasarkan `google_sub`.
2. Kalau belum ada, cari berdasarkan email terverifikasi → **auto-link** (akun password lama tetap bisa login dengan password).
3. Kalau email sudah terhubung ke akun Google lain → `409`.
4. Kalau belum ada sama sekali → buat akun baru tanpa password, `profile_completed=false`.

Sukses → **200** (login) atau **201** (akun baru):

```json
{
  "success": true,
  "message": "login berhasil",
  "data": {
    "access_token": "...", "refresh_token": "...", "token_type": "Bearer", "expires_in": 900,
    "user": { "id": 12, "email": "siswa@gmail.com", "school": "", "grade": 0, "major": "",
              "auth_provider": "google", "profile_completed": false },
    "needs_onboarding": true,
    "auth_provider": "google"
  }
}
```

`needs_onboarding: true` → aplikasi wajib menampilkan layar **Sekolah / Kelas / Jurusan** sebelum masuk beranda. Token tetap diterbitkan supaya user bisa melanjutkan onboarding kapan pun tanpa login ulang.

Akun Google **tidak punya password**: `POST /auth/login` untuk akun seperti ini dibalas `401 "Akun ini terdaftar lewat Google…"`, dan reset password tidak mengirim email (tetap balasan generik).

### 1.8 Lengkapi profil (onboarding)

```
PATCH /users/me/onboarding
{ "school": "SMK Telkom Sidoarjo", "grade": 10, "major": "SIJA" }
```

Wajib untuk akun Google sebelum memakai fitur AI/statistik. `grade` 7–13. Idempotent — boleh dipanggil ulang untuk memperbarui.

Selama `profile_completed=false`, endpoint berikut membalas **409**:

`POST /ai/schedule-recommendations/generate`, `POST /ai/reschedule`, `POST /ai/chat`, `POST /ai/material-predictions/generate`, dan seluruh `GET /statistics/*`.

```json
{ "success": false, "message": "Lengkapi profil dulu (sekolah, kelas, jurusan) sebelum memakai fitur ini",
  "errors": { "profile_completed": "false" } }
```

Endpoint lain (profil, mapel, jadwal, tugas, sesi belajar, notifikasi) tetap bisa dipakai. Penjadwalan otomatis di latar belakang juga **melewati** akun yang belum lengkap.

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

**Error validasi (400):** `errors` berisi pesan per field dalam bahasa Indonesia, memakai nama field JSON.

```json
{ "success": false, "message": "Data yang dikirim belum lengkap atau tidak valid",
  "errors": { "school": "wajib diisi", "grade": "wajib diisi" } }
```

Di **production** detail internal (nama struct Go, tag validator) tidak pernah dikirim; di **development** ditambah `errors.body` berisi error mentah untuk debugging.

**Kode status yang umum:**

| Kode | Arti |
| --- | --- |
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

**PATCH bersifat parsial:** kirim hanya field yang ingin diubah. Field yang tidak dikirim tetap seperti sebelumnya (tidak direset ke kosong). Berlaku untuk `PATCH /api/users/me`, `/subjects/{id}`, `/schedules/{id}`, `/tasks/{id}`, `/activities/{id}`.

```json
PATCH /api/activities/12   { "title": "Latihan Robotik Persiapan Lomba" }
```

**Pagination** (endpoint GET yang bisa panjang): kirim `?page=1&per_page=20`. Balasan `meta`:

```json
"meta": { "page": 1, "perPage": 20, "total": 42, "totalPages": 3 }
```

---

## 3. Profil & Preferensi

### Ringkasan

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/users/me` | Ambil profil sendiri |
| PATCH | `/users/me` | Ubah nama / email / sekolah / grade / major |
| POST | `/users/avatar` | Unggah foto profil siswa (multipart/form-data, maks 2MB, JPG/PNG/WEBP) |
| POST | `/users/change-password` | Ganti password akun & cabut seluruh sesi lama |
| GET | `/users/me/preferences` | Ambil preferensi belajar |
| PATCH | `/users/me/preferences` | Simpan preferensi belajar |

### Contoh: Unggah Foto Profil (Avatar)

```http
POST /api/users/avatar
Authorization: Bearer <access_token>
Content-Type: multipart/form-data
```

Payload (Form-Data):

- `avatar`: file gambar (JPG, JPEG, PNG, WEBP, maks 2MB)

Response: `200 OK`

```json
{
  "success": true,
  "message": "avatar berhasil diperbarui",
  "data": {
    "avatar_url": "/uploads/avatars/avatar_55_1793933940.jpg"
  }
}
```

File statis avatar dapat diakses publik melalui:
`GET http://localhost:8080/uploads/avatars/...`

### Contoh: ganti password akun

```
POST /users/change-password
```

Header:
`Authorization: Bearer <access_token>`

```json
{
  "old_password": "rahasia123",
  "new_password": "passwordBaru456",
  "confirm_password": "passwordBaru456"
}
```

> Catatan: Field password lama mendukung nama JSON `old_password` atau `current_password`. Minimal panjang password baru adalah 8 karakter.

Response Sukses (**200 OK**):

```json
{
  "success": true,
  "message": "password berhasil diubah, silakan login kembali dengan password baru",
  "data": null
}
```

> Seluruh sesi lama (`refresh_tokens`) akan dicabut otomatis demi keamanan akun.

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
| --- | --- | --- |
| `start_time` / `end_time` | `HH:MM` | Boleh dikirim sebagian (PATCH parsial) |
| `session_duration` | angka menit | 15–600. Dipakai AI scheduler sebagai durasi sesi |
| `active_days` | string | `1,2,3,4,5` = Senin–Jumat (1=Senin, 7=Minggu) |

🌱 **Sebelum user mulai belajar**, preferensi ini penting — AI scheduler sangat bergantung padanya.

---

## 4. Mata Pelajaran (Subjects)

### Ringkasan

| Method | Path | Fungsi |
| --- | --- | --- |
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
| --- | --- | --- |
| `name` | ✅ | 1–100 karakter |
| `color` | | hex, mis. `#FF5252` |
| `difficulty` | | 1 (mudah) – 5 (sangat sulit). Dipakai priority engine |

---

## 5. Jadwal Sekolah (Schedules) + Deteksi Bentrok

### Ringkasan

| Method | Path | Fungsi |
| --- | --- | --- |
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
| --- | --- |
| `end_time <= start_time` | 422 |
| `day_of_week` selain `Monday`–`Sunday` | 400 |
| `subject_id` bukan milik user | 404 |

---

## 6. Tugas (Tasks) — jantung aplikasi

### Ringkasan

| Method | Path | Fungsi |
| --- | --- | --- |
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
| --- | --- | --- |
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
| --- | --- | --- |
| `status` | `?status=pending` | Hanya pending |
| `subject_id` | `?subject_id=5` | Hanya mapel 5 |
| `search` | `?search=integral` | Cari di judul (case-insensitive) |
| `sort=priority` | `?sort=priority` | Urut skor prioritas tertinggi dulu (default: deadline terdekat) |
| `page` / `per_page` | `?page=2&per_page=10` | Pagination |

---

## 7. Subtugas (Subtasks)

Memecah tugas besar jadi langkah kecil.

| Method | Path | Fungsi |
| --- | --- | --- |
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
| --- | --- | --- |
| GET | `/activities/` | Daftar aktivitas |
| POST | `/activities/` | Tambah |
| PATCH | `/activities/{id}` | Ubah |
| DELETE | `/activities/{id}` | Hapus |

```json
POST /activities/
{ "title": "Latihan Basket", "start_time": "16:00", "end_time": "17:30", "recurrence": "weekly", "day_of_week": "Wednesday" }
```

`recurrence`: `daily`, `weekly` (default), `monthly`, `none`.
`day_of_week` (opsional, wajib untuk `weekly` agar akurat): `Monday`..`Sunday` / `Senin`..`Minggu` / `1`..`7`. Backend menormalkan nilainya (`Rabu` → `Wednesday`) dan menyimpannya; kalau kosong, engine AI menganggap kegiatan berlangsung **setiap hari**.

`PATCH /activities/{id}` boleh hanya berisi field yang berubah, mis. `{ "day_of_week": "Thursday" }`.

---

## 9. AI Bridge — Rekomendasi & Prediksi

Bagian ini menghubungkan aplikasi ke AI Stoodify. **Client tidak pernah memanggil AI langsung** — semua lewat backend (API key aman di server).

### 9.1 Generate rekomendasi sesi belajar

```
POST /ai/schedule-recommendations/generate
```

Body opsional: `{ "date": "2026-10-10" }` (default: hari ini).

Server mengambil tugas aktif user, meminta AI menyusun jadwal belajar di slot kosong, lalu **langsung menyimpan hasilnya sebagai sesi belajar** (`study_sessions`).

> ℹ️ **Jadwal juga disusun otomatis.** Setiap kali tugas/jadwal/aktivitas/preferensi berubah, backend menyusun ulang jadwal di belakang layar (debounce 3 detik, satu proses per user). Aplikasi cukup me-refresh `GET /study-sessions/`. Endpoint ini adalah **tombol manual** "Susun ulang" dan dibatasi **1× per 10 menit per user**:
> - `429` + `Retry-After` → `message` berisi "Jadwal kamu baru saja diperbarui. Coba lagi dalam N menit…"
> - `200` + "Jadwal kamu sudah paling pas…" → hasil AI identik, tidak ada yang ditulis.
> - `201` → jadwal benar-benar berubah.
>
> Sesi hasil susunan AI diberi `ai_source` (`ai`/`fallback`) dan `ai_warnings`; sesi yang diubah user (`PATCH /study-sessions/:id`) menjadi `ai_generated=false` dan **tidak akan digeser** oleh penyusunan otomatis berikutnya.

Sukses → **201**, `data` berisi array sesi:

```json
{
  "success": true,
  "message": "rekomendasi sesi belajar dibuat",
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
      "reason": "Deadline besok dan tingkat kesulitan tinggi (5)"
    }
  ]
}
```

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

### 9.4 Kalau AI sedang bermasalah

Backend **tidak punya rekomendasi cadangan lokal** — semua keputusan ada di service AI. Tangani di aplikasi seperti ini:

| Respons | Arti | Yang sebaiknya ditampilkan |
| --- | --- | --- |
| `201` | Berhasil (bisa jadi hasil normal atau rencana degradasi dari AI) | Tampilkan sesi seperti biasa |
| `429` + header `Retry-After` | Kuota AI habis / sedang dibatasi | "AI sedang sibuk, coba lagi dalam N detik" lalu tombol coba ulang |
| `503` | Model AI belum siap | Sama seperti di atas, sarankan coba beberapa saat lagi |
| `502` | AI tidak bisa dihubungi / jawabannya tidak valid | "Layanan AI tidak tersedia", sediakan tombol coba lagi dan input manual |

Pada `429`/`503`, isi `message` berasal langsung dari service AI sehingga bisa ditampilkan apa adanya.

### 9.5 Urungkan perubahan jadwal

```
POST /ai/schedule-recommendations/undo
```

Mengembalikan jadwal hari itu ke kondisi sebelum penyusunan otomatis terakhir (snapshot disimpan backend sebelum setiap perubahan).

- `200` → jadwal lama dipulihkan, `data` berisi sesi hasil pemulihan.
- `404` → tidak ada snapshot (belum pernah disusun ulang, atau sudah kedaluwarsa > 7 hari).

Dipakai untuk tombol **Urungkan** pada toast "Jadwal diperbarui".

---

## 10. Notifikasi

Dibuat otomatis oleh scheduler (H-1 & H-0 deadline). Aplikasi cukup menampilkan.

| Method | Path | Fungsi |
| --- | --- | --- |
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
| --- | --- | --- |
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
| --- | --- | --- |
| GET | `/statistics/overview` | Total tugas, selesai, terlambat, % tepat waktu, total jam belajar, jumlah mapel, streak aktif |
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
  "total_subjects": 4,
  "streak_days": 3
}
```

💡 **Catatan Streak:** Dihitung dari kontinuitas hari aktif (tugas selesai atau sesi belajar selesai) dengan *weekend grace period* (toleransi akhir pekan agar streak tidak mudah putus pada siswa).

---

## 13. Kurikulum & Silabus Bertingkat (RoadmapView)

### `GET /curriculum/roadmap/:subjectId`

Mengambil daftar topik silabus mata pelajaran secara terurut beserta status progress belajar siswa (`not_started`, `in_progress`, `mastered`) dan kalkulasi persentase kelulusan (`completion_rate`).

- **Auth**: Wajib (Bearer Token)
- **Header**: `Authorization: Bearer <access_token>`
- **Response**: `200 OK`

  ```json
  {
    "success": true,
    "message": "roadmap kurikulum berhasil diambil",
    "data": {
      "subject_id": 1,
      "subject_name": "Matematika",
      "total_topics": 10,
      "mastered_topics": 4,
      "in_progress_topics": 2,
      "completion_rate": 40.0,
      "topics": [
        {
          "id": 1,
          "subject_id": 1,
          "title": "Integral Substitusi & Parsial",
          "description": "Pengenalan teknik integral tingkat lanjut",
          "order_index": 1,
          "grade": 12,
          "status": "mastered",
          "notes": "Sudah paham konsep dasar",
          "completed_at": "2026-10-30T10:00:00Z"
        }
      ]
    }
  }
  ```

### `POST /curriculum/topics`

Menambahkan topik silabus baru ke mata pelajaran.

- **Auth**: Wajib (Bearer Token)
- **Header**: `Authorization: Bearer <access_token>`
- **Body**:

  ```json
  {
    "subject_id": 1,
    "title": "Integral Tentu & Luas Daerah",
    "description": "Penerapan integral untuk menghitung luas di bawah kurva",
    "order_index": 2,
    "grade": 12,
    "major": "IPA",
    "term": 1,
    "estimated_minutes": 90
  }
  ```

- **Response**: `201 Created`

### `POST /curriculum/topics/:topicId/progress`

Memperbarui status progress belajar siswa untuk topik silabus tertentu.

- **Auth**: Wajib (Bearer Token)
- **Header**: `Authorization: Bearer <access_token>`
- **Body**:

  ```json
  {
    "status": "mastered",
    "notes": "Latihan soal telah diselesaikan"
  }
  ```

- **Response**: `200 OK`

---

## 14. Alur Nyata di Aplikasi (cheat sheet)

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

> ⚠️ Endpoint ini **lokal, bukan AI**: sesi `planned` dijejerkan ulang berurutan dari `preferences.start_time` (durasi = `session_duration`). Jadwal sekolah & aktivitas rutin belum diperhitungkan, jadi tetap tampilkan sebagai usulan yang bisa diubah siswa.

---

## 15. Lampiran: Sumber AI

| Variabel | Fungsi |
| --- | --- |
| `AI_SCHEDULER_URL` | Endpoint AI Spec 1 (penyusun jadwal) |
| `AI_PREDICTOR_URL` | Endpoint AI Spec 2 (prediksi materi) |
| `AI_API_KEY` | Api key bearer untuk kedua endpoint (opsional) |
| `AI_TIMEOUT_SECONDS` | batas tunggu panggilan AI dalam detik (default `15`) |
| `AI_CURRICULUM` | acuan kurikulum untuk prediksi materi (default `Kurikulum Merdeka`) |

AI memberi rekomendasi, **siswa tetap yang memutuskan** (bisa tolak/ubah). Setiap output AI wajib menyertakan alasan (`reason`) yang mudah dipahami siswa — tampilkan field itu apa adanya.
