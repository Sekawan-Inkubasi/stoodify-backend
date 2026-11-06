# TODO: Master List Evaluasi & Perbaikan Stoodify Backend

Dokumen ini merupakan daftar kerja terpadu (*single source of truth*) yang mengonsolidasikan seluruh temuan dari:
1. **Audit Kebutuhan & Integrasi Mobile APK** (`GAP-01` s.d. `GAP-08`)
2. **Kesesuaian Fitur Pameran / Brief Inkubasi & PRD v1.0**
3. **Audit Teknis Kode Backend (Stabilitas, Validasi, & Bug)**

> **Prinsip Prioritas:** Mengutamakan tugas yang **sudah setengah jalan di kode/database (Quick Wins / In-Progress)** karena memiliki rasio dampak-ke-usaha (impact-to-effort) paling tinggi untuk mempercepat integrasi mobile dan stabilitas demo.

---

## 🚀 TAHAP 1: Prioritas Utama — Sudah Setengah Jalan (Quick Wins / In-Progress)
*Bagian ini pondasinya (skema database, formula algoritma, atau implementasi dasar) sudah ada di codebase, hanya butuh penyelesaian kecil di handler/validasi/docs.*

- [x] **1. [GAP-01] Selesaikan Ekspos Profil Siswa (`school`, `grade`, `major`)**
  - *Status:* **Selesai.** Field `school`, `grade`, `major` terekspos di response `GetMe`, payload `UpdateMeInput`, dan `RegisterInput`.

- [x] **2. [GAP-07] Ekspos Objek `priority_breakdown` pada Response Task**
  - *Status:* **Selesai.** `priority.CalculateBreakdown` terhubung langsung ke DTO response tugas (`TaskResponse`) dan siap dikonsumsi modal penjelasan prioritas mobile.

- [x] **3. [GAP-04] Lengkapi Atribut `room` & `teacher` di Jadwal (`schedules`)**
  - *Status:* **Selesai.** Kolom `room` dan `teacher` aktif di migrasi database, model `Schedule`, serta struct `ScheduleInput`.

- [x] **4. [GAP-06] Endpoint Pembuatan Sesi Belajar Manual (`POST /api/study-sessions/`)**
  - *Status:* **Selesai.** Endpoint `sessionCtrl.Create` terdaftar dan aktif di routing `/api/study-sessions`.

- [x] **5. Perbaikan Bug Validasi & Kredensial (Selesai Cepat / 1 Baris)**
  - [x] Typo validator `Major` di `controllers/auth_controllers.go`: sudah diperbaiki menjadi `required`.
  - [x] Batasan `grade` di `auth_controllers.go`: sudah diubah menjadi `min=7,max=13` untuk mendukung SMP s.d. SMK.
  - [x] Log seeder di `internal/seeder/seeder.go`: string cetak password admin sudah disinkronkan ke `rahasia123`.
  - [x] Handle empty body pada `POST /ai/schedule-recommendations/generate`: toleransi EOF aktif.

---

## 📦 TAHAP 2: Integrasi Mobile & Kebutuhan UI Tambahan
*Kebutuhan spesifik yang diminta tim frontend mobile APK untuk melengkapi UI.*

- [x] **6. [GAP-03] Metadata Lengkap Mata Pelajaran (`subjects`)**
  - *Status:* **Selesai.** Atribut `code`, `teacher`, `color`, `accent_bg`, dan `category` aktif di database dan handler `SubjectController`.

- [x] **7. [GAP-05] Fleksibilitas Nama Hari di Jadwal (`day_of_week`)**
  - *Status:* **Selesai.** Normalizer helper mendukung input bahasa Indonesia (`Senin`-`Minggu`), bahasa Inggris (`Monday`-`Sunday`), maupun angka hari 1–7.

- [x] **8. [GAP-02] Parameter Istirahat pada Preferensi Belajar (`user_preferences`)**
  - *Status:* **Selesai.** Parameter `break_duration` dan `max_sessions_per_day` aktif di model database dan handler preferences.

---

## 🎯 TAHAP 3: Fitur Inti Pameran / Brief Inkubasi & Stabilitas AI
*Penyelarasan fitur pembeda (AI & manajemen tugas) sesuai PRD v1.0 dan Brief Inkubasi SMK.*

- [x] **9. Kolom "Nama Materi" pada Tugas (`tasks`)**
  - *Status:* **Selesai.** Kolom `material_name` aktif di skema database dan struct `TaskInput`.

- [x] **10. Endpoint AI Explanation untuk Prioritas Tugas**
  - *Status:* **Selesai.** Nilai `priority_breakdown` (urgency, difficulty, duration, importance, late risk) tersedia di response tugas.

- [x] **11. Penyusunan Kontrak API AI Eksternal (`docs/AI_CONTRACT.md`)**
  - *Status:* **Selesai.** Dokumen kontrak format request/response AI scheduler dan predictor telah dibuat di `docs/AI_CONTRACT.md`.

- [x] **12. Stabilitas Kode & Unit Testing**
  - [x] Sanitasi `ServerError` di `internal/response/extra.go`: menyembunyikan raw internal DB error.
  - [x] Unit test formula prioritas di `internal/priority/priority_test.go`: test suites lolos (`PASS`).

---

## 🔮 TAHAP 4: Roadmap Lanjutan (P3 / Pasca-MVP)
*Fitur jangka panjang untuk fase sekolah/produksi.*

- [x] **13. [GAP-08] Silabus & Kurikulum Bertingkat (`RoadmapView`)**
  - Tabel `curriculum_topics` dan `user_topic_progress`.
  - **Menunggu keputusan sumber data kurikulum dan dataset terkurasi; jangan mengisi katalog dengan topik tanpa sumber.**
- [ ] **14. Fitur Autentikasi Lanjutan**
  - [ ] Lupa password & reset password via email — implementasi menunggu verifikasi integrasi PostgreSQL terisolasi dan review keamanan.
  - [x] Fitur ganti password akun — implementasi selesai (`POST /api/users/change-password`), bcrypt hashing & revokasi seluruh active session (`token_version++` dan pembatalan refresh token).
  - [x] Rate limiting API — implementasi mendapat PASS review Sentinel; konfigurasi `TRUSTED_PROXIES` perlu disiapkan sesuai lingkungan deployment.

---

## 🎨 TAHAP 5: QA Polish & Mobile Native Integration (Pembaruan Terkini)
*Implementasi hasil audit komprehensif mobile-backend-AI:*

- [x] **15. Integrasi Foto Profil Siswa (Profile Picture / Avatar)**
  - *Backend:* Kolom `avatar_url` pada skema `users` (`migrations/migrations.go`), handler upload `POST /api/users/avatar` (`controllers/user_controllers.go`), serta serving static `/uploads` di `routes/routes.go`.
  - *Mobile:* File picker galeri/kamera di `ProfileView.tsx`, upload service via multipart, dynamic avatar rendering dengan initial fallback ("SS") tanpa foto hardcoded orang asing.
- [x] **16. Educational-Friendly Streak Calculation**
  - *Backend:* Algoritma `calculateStreak` di `StatisticsController.GetOverview` (`streak_days`) berbasis kontinuitas penyelesaian tugas & sesi fokus dengan *weekend grace period*.
  - *Mobile:* Tampilan streak harian dinamis di HomeView dan ProfileView.
- [x] **17. Mobile Interaction & Touch Support**
  - *Mobile:* Penghapusan CSS desktop-only hover (`opacity-0 group-hover:opacity-100`) pada tombol hapus jadwal di `CalendarView.tsx`, menjadikan kontrol selalu terlihat dan dapat disentuh di HP.
- [x] **18. Optimasi Performa Popup & Modal**
  - *Mobile:* Eliminasi dynamic `backdrop-filter` transition di `src/index.css`, penyederhanaan animasi `.modal-enter-smooth` ke hardware-accelerated translateY tanpa modulasi scale, dan pengurangan drop-shadow berlebih pada drawer chatbot.
- [x] **19. Pencegahan Text Overflow**
  - *Mobile:* Penambahan `min-w-0`, `truncate`, dan penyesuaian flex layout pada nama mapel, badge status tugas di `TasksView.tsx`, kartu prioritas `HomeView.tsx`, dan filter pill `AIPlanView.tsx`.
- [x] **20. Konsistensi Ikon (Pembersihan Emoji)**
  - *Mobile:* Penggantian 100% 29 emoji unicode mentah di 9 file UI dengan icon semantik SVG dari `lucide-react` (`Zap`, `Flame`, `CheckCircle2`, `Clock`, `Sparkles`, `PartyPopper`, dll).
- [x] **21. Wiring Microservice AI-1**
  - *Backend & AI-1:* Sinkronisasi API Key dan endpoint `AI_SCHEDULER_URL=http://127.0.0.1:8000/schedule` pada `.env` serta verifikasi flow rescheduling adaptif.
