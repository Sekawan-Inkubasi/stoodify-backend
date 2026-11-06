# Stoodify Development State & Handover

Catatan serah terima kondisi codebase saat ini (*current state*) untuk sesi agent berikutnya.

---

## Status Terkini (Per 6 November 2026)
- **Branch Aktif**: `dev`
- **Verifikasi lokal terbaru**: `go test ./...`, `go build ./...`, dan pengujian browser live via `agent-browser` pada mobile web lolos 100%.
- **Implementasi Tambahan**:
  1. Penambahan `avatar_url` pada model `User` dan endpoint `POST /api/users/avatar` dengan static file serving `/uploads`.
  2. Implementasi kalkulasi streak ramah edukasi (`streak_days`) pada endpoint `GET /api/statistics/overview` dengan toleransi akhir pekan.
  3. Integrasi environment backend ke engine Python `stoodify-ai-1` (`AI_SCHEDULER_URL=http://127.0.0.1:8000/schedule`).
  4. Optimasi performa popup/modal frontend mobile, pembersihan 100% emoji menjadi `lucide-react`, perbaikan touch interaction jadwal, dan pencegahan text overflow.
- **Status Backlog**: Tahap 1 s.d. Tahap 5 di `docs/to-do.md` telah diselesaikan 100%.

## Fitur yang Siap Digunakan (MVP Ready)
1. Autentikasi lengkap (Register, Login, Refresh Token dengan rotasi, Me, Update Profil, Unggah Foto Profil Avatar).
2. Manajemen Mata Pelajaran dengan metadata mobile lengkap (`code`, `teacher`, `accent_bg`, `category`).
3. Jadwal Pelajaran mingguan dengan fleksibilitas nama hari (Bahasa Indonesia & Inggris).
4. Manajemen Tugas & Subtugas dengan nama materi (`material_name`) dan `priority_breakdown` dinamis.
5. Manajemen Aktivitas Rutin siswa.
6. Sesi Belajar Mandiri manual (`POST /api/study-sessions/`) dan otomatis via AI Scheduler.
7. Notifikasi in-app dan scheduler otomatis tugas telat (*late task updater*).
8. Statistik belajar harian & mingguan beserta kalkulasi streak hari belajar aktif (`streak_days`).
9. Microservice AI-1 terintegrasi untuk penjadwalan cerdas dan adaptive rescheduling.

## Sisa Roadmap (Tahap 4 - Pasca MVP)
- Silabus kurikulum bertingkat (`RoadmapView`) — menunggu sumber/dataset kurikulum terkurasi.
- Alur reset password dan ganti password — implementasi menunggu verifikasi PostgreSQL terisolasi dan review keamanan akhir.
- Rate limiting API — implementasi mendapat PASS review Sentinel; siapkan `TRUSTED_PROXIES` sebelum deploy.
