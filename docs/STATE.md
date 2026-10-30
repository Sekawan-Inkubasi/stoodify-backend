# Stoodify Development State & Handover

Catatan serah terima kondisi codebase saat ini (*current state*) untuk sesi agent berikutnya.

---

## Status Terkini (Per 30 Oktober 2026)
- **Branch Aktif**: `dev`
- **Verifikasi lokal terbaru**: `go test -race ./...`, `go vet ./...`, `go build ./...`, dan `git diff --check` lolos; tes integrasi password PostgreSQL **di-skip** jika `TEST_DATABASE_URL` tidak disetel.
- **CI**: Workflow menyediakan service PostgreSQL 16 sementara dan `TEST_DATABASE_URL` khusus untuk menjalankan tes integrasi; hasil CI belum diverifikasi.
- **Tahap 4**: Rate limiter mendapat PASS dari Sentinel. Reset/ganti password masih menunggu verifikasi PostgreSQL terisolasi dan review akhir. Kurikulum menunggu keputusan sumber/dataset terkurasi.
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
- Silabus kurikulum bertingkat (`RoadmapView`) — menunggu sumber/dataset kurikulum terkurasi.
- Alur reset password dan ganti password — implementasi menunggu verifikasi PostgreSQL terisolasi dan review keamanan akhir.
- Rate limiting API — implementasi mendapat PASS review Sentinel; siapkan `TRUSTED_PROXIES` sebelum deploy.
