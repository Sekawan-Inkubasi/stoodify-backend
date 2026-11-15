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
