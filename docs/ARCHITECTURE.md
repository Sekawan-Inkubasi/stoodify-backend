# Stoodify Backend Architecture

Dokumen ini menjelaskan diagram sistem, batasan domain, dan pola arsitektur utama pada backend Stoodify.

---

## 1. Diagram Sistem & Alur Kerja

```
+-------------------------+         +----------------------------+
|  Mobile APK / Frontend  | <-----> |   Gin HTTP REST Layer      |
+-------------------------+  JSON   +--------------+-------------+
                                                   |
         +-----------------------------------------+
         |
         v
+-------------------+      +--------------------+      +--------------------+
| Controllers Layer | ---> | Internal Engines   | <--> | PostgreSQL (Neon / |
| (Bind & Validate) |      | (Priority, Token)  |      | Supabase)          |
+-------------------+      +--------------------+      +--------------------+
         |
         +--------------------------+
                                    v
                     +------------------------------+
                     |   AI Service Specialist      |
                     | (FastAPI / Gemini Assistant) |
                     +------------------------------+
```

---

## 2. Batasan Domain (Domain Boundaries)

1. **Auth & Identity Domain**
   - Menangani registrasi, login, refresh token rotasi, dan profil siswa.
   - Token: JWT Access token (15 menit) + Refresh token (7 hari tersimpan hash-nya di DB).

2. **Academic Domain (Subjects & Schedules)**
   - Mata pelajaran siswa dan jadwal mingguan sekolah.
   - Normalizer nama hari otomatis memetakan nama hari lokal (*Senin*) ke representasi standar (*Monday*).

3. **Task & Priority Domain**
   - Manajemen tugas sekolah, subtasks, deadline, dan kesulitan.
   - Terhubung dengan `internal/priority` yang menghitung bobot urgensi, durasi, dan risiko terlambat.
   - Nilai `priority_breakdown` dihitung dinamis (*computed on the fly*) untuk modal detail di mobile.

4. **AI & Study Session Domain**
   - Penjadwalan sesi belajar mandiri dan prediksi materi berikutnya.
   - Memiliki mekanisme fallback rule-based otomatis jika service AI eksternal offline atau lambat (>5 detik).

---

## 3. Pola Kepemilikan Data (Multi-Tenant Isolation)

Setiap siswa hanya boleh mengakses dan mengubah data miliknya sendiri. Pola standar GORM:
```go
user := currentUser(c)
query := ctrl.DB.WithContext(c.Request.Context()).Where("user_id = ?", user.ID)
```
Jika data tidak ditemukan dengan kombinasi `id` dan `user_id`, kembalikan `404 Not Found`.
