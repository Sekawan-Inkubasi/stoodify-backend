# Stoodify Development State & Handover

Catatan serah terima kondisi codebase saat ini (*current state*) untuk sesi agent berikutnya.

---

## Status Terkini (Per 19 November 2026)
- **Branch Aktif**: `dev` (identik dengan `main` kecuali workflow CI: dev pakai `ci.yml` tanpa job deploy, main pakai `ci-cd.yml` dengan deploy VPS).
- **Verifikasi lokal terbaru**: `go build ./...`, `go vet ./...`, `go test ./...` hijau, dan `gofmt -l .` kosong (gate CI memakai `test -z "$(gofmt -l .)"`).
- **Implementasi Tambahan**:
  1. Penambahan `avatar_url` pada model `User` dan endpoint `POST /api/users/avatar` dengan static file serving `/uploads`. Catatan: file disimpan di disk lokal container.
  2. Implementasi kalkulasi streak ramah edukasi (`streak_days`) pada endpoint `GET /api/statistics/overview` dengan toleransi akhir pekan.
  3. Integrasi environment backend ke engine Python `stoodify-ai-1` dan `stoodify-ai-2` (`AI_SCHEDULER_URL=http://127.0.0.1:8000/schedule`, `AI_PREDICTOR_URL=http://127.0.0.1:8000/predict`) untuk generate jadwal dan prediksi materi. Backend bersifat **bridge murni**: fallback rule-based sudah dihapus seluruhnya (ADR-005), `serve` menolak start kalau URL AI kosong, dan `POST /api/ai/reschedule` masih murni lokal (tidak memanggil AI).
  4. Parse respons AI bersifat toleran: menerima gaya kontrak (`session_date`, `confidence_score`, `prerequisites` array, wrapper `data`) maupun gaya internal. Detail di `docs/AI_CONTRACT.md`.
  5. Status/limit AI diteruskan ke klien: `429`/`503` + `Retry-After` dari Python dikembalikan apa adanya, sisanya `502`. Keputusan degradasi ada di Python, bukan di Go.
  6. **Pembersihan Sesi Planned Otomatis (ADR-006):** Pada `controllers/ai_controllers.go:GenerateSchedule`, sesi planned usang dibersihkan sebelum menyimpan rekomendasi baru dari AI untuk mencegah penumpukan duplikat sesi di kalender. Sesi berstatus `skipped` difilter keluar dari `GetScheduleRecommendations`.
  7. **Fleksibilitas Masa Berlaku Token (ADR-007):** `internal/token/token.go` mendukung kustomisasi `JWT_ACCESS_TTL` dan `JWT_REFRESH_TTL` via environment variable (default 15m dan 7d).
  8. Perubahan frontend mobile (dialog modern Stoodify, ChatDrawer anti-overlap & backdrop overlay, keyboard avoiding view, deduplikasi kalender, dan proactive token refresh) berada di repo `stoodify-apk`.
- **Status Backlog**: seluruh item di `docs/to-do.md` Tahap 1–6 selesai, seluruh temuan gap integrasi mobile (`GAP-01` s.d. `GAP-08`) selesai 100%, kecuali: reset password (kode selesai, verifikasi end-to-end belum), dataset kurikulum (endpoint siap, katalog topik sengaja kosong), dan rescheduling adaptif berbasis AI.

## Fitur yang Siap Digunakan (MVP Ready)
1. Autentikasi lengkap (Register, Login, Refresh Token dengan rotasi, Me, Update Profil, Unggah Foto Profil Avatar, Ganti Password, Lupa/Reset Password via email, konfigurasi fleksibel TTL token).
2. Manajemen Mata Pelajaran dengan metadata mobile lengkap (`code`, `teacher`, `accent_bg`, `category`).
3. Jadwal Pelajaran mingguan dengan fleksibilitas nama hari (Bahasa Indonesia & Inggris).
4. Manajemen Tugas & Subtugas dengan nama materi (`material_name`) dan `priority_breakdown` dinamis.
5. Manajemen Aktivitas Rutin siswa.
6. Sesi Belajar Mandiri manual (`POST /api/study-sessions/`) dan otomatis via AI Scheduler (dengan auto-cleanup sesi planned usang).
7. Notifikasi in-app dan scheduler otomatis tugas telat (*late task updater*).
8. Statistik belajar harian & mingguan beserta kalkulasi streak hari belajar aktif (`streak_days`).
9. Microservice AI terintegrasi untuk generate jadwal dan prediksi materi 100% LLM (kontrak di `docs/AI_CONTRACT.md`).
10. Kurikulum & silabus bertingkat: endpoint roadmap/topik/progres aktif, katalog topik menunggu dataset terkurasi.
11. Rate limiting API (60 req/menit umum, 10 req/menit untuk auth) — isi `TRUSTED_PROXIES` sebelum deploy di balik proxy.

## Sisa Roadmap
- **Rencana degradasi di sisi Python**: backend sudah murni bridge. Kalau service AI mati atau LLM limit, fitur jadwal & prediksi mengembalikan `502`/`429` (tidak ada jadwal cadangan). Rencana degradasi/cache harus dibuat di service AI.
- **Rescheduling adaptif berbasis AI**: `POST /api/ai/reschedule` masih menyusun ulang sesi secara lokal (berurutan dari `start_time` preferensi) dan mengabaikan jadwal sekolah/aktivitas. Perlu keputusan produk sebelum diubah.
- **Dataset kurikulum**: endpoint siap, tetapi katalog topik sengaja belum diisi (jangan isi topik tanpa sumber).
- **Gate format**: sudah setara di dua branch — `dev` (`ci.yml`) dan `main` (`ci-cd.yml`, diperbaiki pada merge `0a6839d`) sama-sama memakai `test -z "$(gofmt -l .)"` + `go test -race ./...` + `go vet ./...` sebelum docker build.
- **Penyimpanan avatar**: masih disk lokal (`uploads/avatars`), belum object storage; hilang saat redeploy di platform ephemeral.
