# TODO Divisi Mobile — Penjadwalan Otomatis

Scope: aplikasi mobile (`stoodify-mobile` / `stoodify-apk`). Backend tidak mengerjakan item di sini.
Rujukan kontrak backend: [`docs/PLAN-AUTO-SCHEDULE.md`](../PLAN-AUTO-SCHEDULE.md) dan [`docs/API.md`](../API.md).

Latar: jadwal AI sekarang **diterapkan otomatis** oleh backend setiap ada perubahan tugas. Tidak ada lagi alur "AI mengusulkan → user menerima". UI harus menyesuaikan agar jadwal terasa milik user.

---

## 1. Hapus alur terima/tolak

- [ ] Hapus tombol **Terima** dan **Tolak** per sesi di `AIPlanView.tsx`.
- [ ] Hapus state/badge "menunggu persetujuan"; semua sesi yang tampil sudah final.
- [ ] Hapus pemanggilan `POST /ai/schedule-recommendations/{id}/accept` dan `/reject` dari client.
- [ ] Ganti label layar dari "Rekomendasi AI" menjadi **"Jadwal Belajar"**.
- [ ] Hapus loading/empty state khusus "AI sedang menyusun rekomendasi"; ganti dengan skeleton biasa.

**Acceptance:** tidak ada teks/tombol yang menyiratkan user harus menyetujui jadwal.

## 2. Toast + Urungkan

- [ ] Saat aplikasi mendeteksi jadwal berubah (refetch setelah simpan tugas), tampilkan toast non-blocking: **"Jadwal diperbarui karena ada tugas baru"**.
- [ ] Toast punya dua aksi: **[Lihat]** (scroll ke sesi hari ini) dan **[Urungkan]** (panggil `POST /api/ai/schedule-recommendations/undo`).
- [ ] Auto-dismiss ±10 detik; kalau user menekan Urungkan setelah kedaluwarsa, tampilkan pesan "Waktu membatalkan sudah lewat".
- [ ] Jangan pakai modal/dialog konfirmasi untuk perubahan jadwal otomatis.
- [ ] Kalau `undo` gagal (404 karena snapshot habis), tampilkan: "Jadwal sebelumnya sudah tidak tersedia".

**Acceptance:** perubahan jadwal otomatis tidak pernah memblokir interaksi user.

## 3. Penanda sesi

- [ ] Beri label halus **"baru"** pada sesi yang baru dibuat/digeser (bandingkan `updated_at`/`plan_version`).
- [ ] Tampilkan ikon kecil **"diubah"** pada sesi hasil editan user (pinned).
- [ ] Kalau `ai_source = "fallback"`, tampilkan keterangan netral: **"Jadwal disusun dari rencana cadangan"** (jangan sebut "AI gagal").

**Acceptance:** user bisa membedakan sesi otomatis vs sesi yang dia ubah sendiri tanpa membaca dokumentasi.

## 4. Edit sesi = pin

- [ ] Saat user mengubah jam/tanggal sesi (`PATCH /api/study-sessions/:id`), backend otomatis menandainya pinned — client hanya perlu mengirim perubahan seperti biasa.
- [ ] Setelah edit, tampilkan toast: **"Sesi ini kamu atur sendiri — tidak akan diubah otomatis."**
- [ ] Jelaskan sekali (info tooltip/first-time hint) bahwa sesi yang diubah tidak ikut disusun ulang.

**Acceptance:** mengedit sesi tidak memicu peringatan/konflik apa pun, dan sesi itu tetap di tempatnya setelah regenerasi berikutnya.

## 5. Tombol "Susun ulang" manual

- [ ] Tombol tetap ada, tapi hanya untuk kasus user ingin menyusun ulang seluruh hari.
- [ ] Tangani `429` + `Retry-After`: tampilkan pesan dari backend apa adanya (`message`), plus hitung mundur.
- [ ] Tangani `200` "tidak ada yang berubah": tampilkan **"Jadwal kamu sudah paling pas"**.
- [ ] Tangani `502`/`503`: **"AI sedang sibuk. Jadwal lama tetap dipakai."** tanpa menghapus tampilan jadwal.
- [ ] Nonaktifkan tombol sementara selama request berjalan (hindari double tap).

**Acceptance:** spam tombol tidak menghasilkan lebih dari satu request efektif dan pesannya jelas.

## 6. Sinkronisasi data

- [ ] Refetch `GET /api/study-sessions/?status=planned` setelah setiap simpan/ubah/hapus tugas, ubah jadwal sekolah, aktivitas, dan preferensi.
- [ ] Refetch saat app kembali dari background (resume) dan saat pull-to-refresh.
- [ ] Tidak perlu websocket/push untuk MVP.

**Acceptance:** jadwal yang tampil selalu ≤ 1 aksi user dari kondisi server.

## 7. Data baru yang perlu ditampilkan/dipakai

- [ ] `ai_source` (opsional): untuk label cadangan di poin 3.
- [ ] `ai_warnings` (opsional): sembunyikan dari user, cukup untuk debugging internal.
- [ ] `day_of_week` pada form aktivitas (dropdown Senin–Minggu) — tanpa ini kegiatan mingguan tidak akurat.

**Acceptance:** form aktivitas mengirim `day_of_week` saat recurrence `weekly`.

## 8. Empty & error state

- [ ] Belum ada tugas → "Belum ada tugas. Tambah tugas pertama kamu, jadwal belajar akan dibuat otomatis."
- [ ] Tugas ada tapi jadwal kosong (AI belum selesai) → skeleton + teks "Sedang menyusun jadwal…" (maks 5 detik), lalu fallback ke teks netral tanpa menyalahkan AI.

## 9. Pengujian

- [ ] Simpan 4 tugas beruntun → UI hanya menampilkan satu toast pembaruan.
- [ ] Matikan service AI → simpan tugas → jadwal lama tetap tampil, tidak ada error dialog.
- [ ] Tekan "Susun ulang" dua kali cepat → hanya satu request yang efektif, pesan limit tampil benar.
- [ ] Edit sesi → regenerasi (simpan tugas lain) → sesi hasil edit tidak berpindah.
- [ ] Tekan "Urungkan" → jadwal kembali ke kondisi sebelum perubahan.

---

## Catatan koordinasi

- Endpoint `accept`/`reject` **masih ada** di backend untuk sementara (deprecated). Jangan dipakai lagi, tapi jangan panik kalau masih muncul di collection Bruno lama.
- Kontrak payload/response tidak berubah selain tambahan field opsional (`ai_source`, `ai_warnings`, `plan_version`).
- Perubahan urutan kerja: backend fase 1–3 dulu, mobile bisa mulai poin 1, 2, 5, 6 paralel karena tidak menunggu endpoint baru (kecuali `undo`).
