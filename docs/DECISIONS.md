# Architecture Decision Records (ADR)

Catatan pertimbangan dan keputusan desain arsitektural teknis Stoodify Backend.

---

## ADR-001: Komputasi Dinamis untuk `priority_breakdown`
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Tim mobile APK membutuhkan rincian sub-skor (urgency score, difficulty score, duration score, late risk score) untuk modal dialog alasan prioritas tugas.
- **Keputusan:** Tidak menambahkan kolom baru pada tabel `tasks`. Sub-skor dihitung saat response dibentuk (*on the fly*) melalui struct `TaskResponse` dan package `internal/priority`.
- **Konsekuensi:** Menghemat skema database dan mencegah masalah sinkronisasi nilai (*stale data*) ketika waktu berjalan mendekati deadline.

---

## ADR-002: Standar Koleksi API Client Bruno Menggunakan OpenCollection YAML
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Format lawas Bruno `.bru` sering kali tidak terbaca di Bruno UI versi 4 yang mengadopsi spesifikasi OpenCollection v1.0.0.
- **Keputusan:** Seluruh spesifikasi request di `docs/collection_bruno/` disimpan dalam format `.yml` dengan manifest `opencollection.yml`.
- **Konsekuensi:** Kompatibel penuh dengan GUI Bruno v4+ dan mudah di-parse oleh script tanpa parser proprietary.

---

## ADR-003: Graceful Fallback untuk Rekomendasi AI
- **Status:** **Superseded** oleh ADR-005 (13 November 2026)
- **Konteks (historis):** Ketergantungan pada model AI eksternal rentan mengalami rate limit, timeout, atau jaringan putus saat demo pameran.
- **Keputusan (historis):** Backend mengimplementasikan rule-based heuristic scheduler internal di controller AI yang otomatis aktif saat panggilan AI eksternal gagal atau payload kosong.
- **Kenapa diganti:** dua sumber kebenaran (Go dan Python) membuat hasil sulit diprediksi, dan fallback hanya menjejalkan tugas ke rentang preferensi tanpa mempertimbangkan jadwal sekolah/aktivitas. Penanganan limit kini dipusatkan di service Python.

---

## ADR-005: Backend AI Bersifat Bridge Murni (tanpa Fallback Lokal)
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Integrasi dengan service AI (`stoodify-ai-1`) perlu satu sumber kebenaran. Perilaku saat LLM kena rate limit/timeout harus bisa diubah tanpa deploy ulang backend Go.
- **Keputusan:**
  1. Hapus total fallback rule-based (`fallbackSlots`, `fallbackPrediction`) dan env `AI_FALLBACK_ENABLED`.
  2. Semua keputusan saat AI bermasalah ada di Python: balas `200` dengan rencana degradasi bila masih punya plan, atau `429`/`503` bila tidak.
  3. Backend hanya meneruskan: `429`/`503` + `Retry-After` dan pesan Python dikembalikan apa adanya; status lain, timeout, atau koneksi gagal menjadi `502` generik.
  4. `AI_SCHEDULER_URL` dan `AI_PREDICTOR_URL` wajib; `serve` gagal saat boot kalau kosong.
- **Konsekuensi:** kalau service AI mati, fitur jadwal & prediksi mengembalikan error (tidak ada jadwal cadangan). Ini disengaja: rencana degradasi harus dibuat di Python. Sesi/prediksi parsial tidak pernah disimpan.

---
## ADR-004: Sanitasi Database Error di Environment Production
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Fungsi `response.ServerError` sebelumnya meneruskan string `err.Error()` mentah ke klien, berisiko membocorkan nama tabel atau credential SQL.
- **Keputusan:** Mengubah helper `ServerError` agar mengembalikan pesan umum yang aman, sementara error asli hanya dicatat ke logger internal.

---

## ADR-006: Pembersihan Sesi Planned Usang & Filter Status Sesi Rekomendasi
- **Status:** Diterima & Diimplementasikan (19 November 2026)
- **Konteks:** Setiap kali user memicu `POST /api/ai/schedule-recommendations/generate`, sesi rekomendasi baru dimasukkan ke database tanpa menghapus sesi `planned` yang telah dihasilkan sebelumnya. Akibatnya, pada tampilan kalender mobile terjadi akumulasi duplikat (hingga 8 sesi identik).
- **Keputusan:**
  1. Pada `controllers/ai_controllers.go:GenerateSchedule`, sebelum memasukkan sesi rekomendasi baru dari AI, hapus secara atomik sesi-sesi berstatus `planned` milik user (`Where("user_id = ? AND status = ?", userID, migrations.SessionPlanned).Delete(...)`).
  2. Pada query `GetScheduleRecommendations`, tambahkan filter `status != ?` untuk mengecualikan sesi dengan status `migrations.SessionSkipped`.
- **Konsekuensi:** Tampilan kalender mobile selalu bersih, mutakhir, dan bebas dari duplikasi sesi planned lama.

---

## ADR-007: Konfigurasi Fleksibel Masa Berlaku Token JWT via Environment Variable
- **Status:** Diterima & Diimplementasikan (19 November 2026)
- **Konteks:** Sebelumnya konstanta `AccessTTL` (15 menit) dan `RefreshTTL` (7 hari) di-hardcode di package `internal/token`. Pengujian mobile dan staging membutuhkan fleksibilitas penyesuaian masa aktif token tanpa perlu mengubah kode sumber atau mengompilasi ulang binary.
- **Keputusan:**
  1. Mengubah konstanta `AccessTTL` dan `RefreshTTL` menjadi variabel package yang diinisialisasi melalui fungsi `init()`.
  2. Membaca `JWT_ACCESS_TTL` dan `JWT_REFRESH_TTL` dari environment variable (format `time.ParseDuration`, misal `30m`, `1h`, `168h`), dengan fallback aman ke nilai standar 15 menit dan 7 hari.
- **Konsekuensi:** Operator dan developer dapat menyesuaikan masa aktif sesi secara dinamis pada environment development maupun production.

---

---

## ADR-008: Penjadwalan Otomatis dengan Planner Internal
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Fitur jadwal hanya berguna kalau selalu mutakhir. Memanggil AI setiap kali user menyimpan tugas tanpa pengaman menghasilkan banyak panggilan LLM (user bisa menambah 4 tugas dalam 30 detik) dan jadwal yang berubah berkali-kali di depan user.
- **Keputusan:**
  1. Setiap perubahan tugas/jadwal/aktivitas/preferensi memicu regenerasi lewat `workers.Planner`: **debounce 3 detik** + **single-flight per user** + flag `dirty` (maksimal satu susulan).
  2. Hasil langsung dipakai (tanpa terima/tolak); yang diubah user (`ai_generated=false`) dan sesi < 24 jam **tidak digeser**.
  3. Kegagalan AI **tidak** mengganggu user: jadwal lama dipertahankan, error hanya di log.
  4. Sebelum menulis, backend menyimpan snapshot untuk fitur **Urungkan**; penulisan dalam satu transaksi (hapus sesi AI `planned` di tanggal itu → insert baru).
  5. Tombol manual "Susun ulang" dibatasi 1×/10 menit per user.
- **Konsekuensi:** keputusan penjadwalan tetap milik AI (bridge-only), backend hanya mengatur ritme, menyimpan, dan menjaga sesi milik user. Horizon masih satu tanggal (hari ini) + rollover 00:05; horizon 7 hari menunggu dukungan parameter di engine.

---

## ADR-009: Login Google dengan Auto-link Email dan Onboarding Profil
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Form daftar meminta sekolah/kelas/jurusan, sedangkan Google hanya memberi nama/email/foto. Menahan token aplikasi sampai form selesai memaksa adanya token onboarding sementara (TTL pendek, jalur resume terpisah) dan berisiko menyisakan akun setengah jadi tanpa sesi.
- **Keputusan:**
  1. `POST /auth/google` memverifikasi ID token sendiri (signature via JWKS Google, `iss`, `aud` = `GOOGLE_CLIENT_ID`, `exp`, `email_verified`), lalu **selalu** menerbitkan access+refresh token aplikasi.
  2. Identitas memakai klaim `sub` (`users.google_sub`, unik) — bukan email, karena email bisa berubah.
  3. Email terverifikasi yang sudah terdaftar **auto-link**: akun password tetap bisa login dengan password; akun Google-only tidak punya password.
  4. Akun baru dibuat dengan `profile_completed=false`; fitur AI/statistik diblok `409` lewat middleware sampai `PATCH /users/me/onboarding` diisi. Penjadwalan otomatis di latar belakang melewati akun ini.
  5. Login password untuk akun Google ditolak dengan pesan eksplisit; reset password tidak mengirim email untuk akun tersebut.
- **Konsekuensi:** butuh kolom `google_sub`, `auth_provider`, `profile_completed` (data lama di-backfill `password`/`true`). Client ID Android/iOS dikelola aplikasi masing-masing; backend hanya menyimpan client ID web.
