# Stoodify Business & Technical Glossary

Daftar istilah domain khusus yang digunakan di seluruh codebase Stoodify.

---

- **Priority Score**: Skor numerik dari 0 hingga 100 yang menentukan urutan urgensi suatu tugas sekolah. Formula mengombinasikan urgensi deadline (40%), kesulitan (20%), durasi (15%), jenis tugas (15%), dan risiko keterlambatan (10%).
- **Priority Breakdown**: Objek sub-skor turunan yang merinci kontribusi masing-masing faktor pembentuk Priority Score.
- **Study Session**: Sesi belajar teralokasi (dengan tanggal, jam mulai, dan durasi) yang ditargetkan untuk mencicil progres tugas tertentu.
- **Late Task**: Tugas yang telah melewati waktu batas pengumpulan (`deadline`) dan secara otomatis diubah statusnya menjadi `late` oleh scheduler.
- **Material Prediction**: Prediksi topik atau materi kurikulum berikutnya yang kemungkinan akan dipelajari di kelas.
- **AI Bridge**: Pola integrasi backend dengan service AI eksternal — backend hanya mengirim konteks dan meneruskan hasil/error, tanpa membuat keputusan cerdas sendiri.
- **Rate-limit pass-through**: `429`/`503` beserta header `Retry-After` dan pesan dari service AI dikembalikan apa adanya ke klien; status lain dipetakan menjadi `502`.
- **OpenCollection**: Standar spesifikasi terbuka file API client (versi 1.0.0) berbasis YAML yang digunakan oleh Bruno v4+.

## Google Sub (`google_sub`)
Klaim `sub` dari Google ID token — ID unik dan stabil per akun Google. Dipakai sebagai kunci identitas OAuth (bukan email, karena email bisa berubah). Kosong berarti akun tidak terhubung ke Google.

## Onboarding Profil (`profile_completed`)
Status kelengkapan sekolah/kelas/jurusan. Akun hasil login Google dibuat dengan `false`; selama `false`, endpoint AI dan statistik membalas `409` sampai `PATCH /users/me/onboarding` diisi. Penjadwalan otomatis di latar belakang melewati akun ini.

## Email Verified (`email_verified`)
Penanda bahwa pemilik email sudah membuktikan kepemilikannya lewat link verifikasi (sekali pakai, 24 jam). Akun dengan nilai `false` dianggap belum punya akun: login dan rotasi refresh token ditolak `403`. Akun Google selalu `true` karena Google sudah memverifikasi emailnya.

