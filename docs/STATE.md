# Stoodify Development State & Handover

Catatan serah terima kondisi codebase saat ini (*current state*) untuk sesi agent berikutnya.

---

## Status Terkini (Per 30 Oktober 2026)
- **Branch Aktif**: `dev`
- **Status Kompilasi**: `go build ./...` lolos bersih tanpa peringatan.
- **Status Unit Test**: `go test ./...` PASS.
- **Status Backlog**: Tahap 1, Tahap 2, dan Tahap 3 di `docs/to-do.md` telah diselesaikan 100%.

## Fitur yang Siap Digunakan (MVP Ready)
1. Autentikasi lengkap (Register, Login, Refresh Token dengan rotasi, Me, Update Profil).
2. Manajemen Mata Pelajaran dengan metadata mobile lengkap (`code`, `teacher`, `accent_bg`, `category`).
3. Jadwal Pelajaran mingguan dengan fleksibilitas nama hari (Bahasa Indonesia & Inggris).
4. Manajemen Tugas & Subtugas dengan nama materi (`material_name`) dan `priority_breakdown` dinamis.
5. Manajemen Aktivitas Rutin siswa.
6. Sesi Belajar Mandiri manual (`POST /api/study-sessions/`) dan otomatis via AI Scheduler.
7. Notifikasi in-app dan scheduler otomatis tugas telat (*late task updater*).
8. Statistik belajar harian & mingguan.

## Sisa Roadmap (Tahap 4 - Pasca MVP)
- Silabus kurikulum bertingkat (`RoadmapView`).
- Alur reset password akun via email.
- Rate limiting API.
