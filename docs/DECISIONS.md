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
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Ketergantungan pada model AI eksternal rentan mengalami rate limit, timeout, atau jaringan putus saat demo pameran.
- **Keputusan:** Backend mengimplementasikan rule-based heuristic scheduler internal di controller AI yang otomatis aktif saat panggilan AI eksternal gagal atau payload kosong.
- **Konsekuensi:** Siswa tetap mendapatkan jadwal belajar terstruktur meskipun layanan AI sedang tidak dapat dijangkau.

---
## ADR-004: Sanitasi Database Error di Environment Production
- **Status:** Diterima & Diimplementasikan
- **Konteks:** Fungsi `response.ServerError` sebelumnya meneruskan string `err.Error()` mentah ke klien, berisiko membocorkan nama tabel atau credential SQL.
- **Keputusan:** Mengubah helper `ServerError` agar mengembalikan pesan umum yang aman, sementara error asli hanya dicatat ke logger internal.
