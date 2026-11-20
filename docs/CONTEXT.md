# Stoodify Project Context

Latar belakang produk, audiens sasaran, dan konteks pameran inkubasi.

---

## 1. Mengenai Stoodify
Stoodify adalah aplikasi asisten belajar harian berbasis kecerdasan buatan (AI) yang dirancang khusus untuk siswa SMA dan SMK kelas 10–13 di Indonesia (rentang `grade` yang diterima backend: 10–13). Masalah utama yang dipecahkan adalah penumpukan tugas sekolah, ketidakmampuan menentukan prioritas belajar, dan bentrok antara jadwal sekolah dengan aktivitas pribadi.

## 2. Fitur Pembeda Utama (Key Differentiators)
1. **Dynamic Priority Scoring Engine**: Menghitung secara otomatis urutan pengerjaan tugas berdasarkan 5 faktor (urgensi deadline, tingkat kesulitan, estimasi durasi, bobot jenis tugas, dan risiko terlambat).
2. **AI Study Scheduler**: Menyusun slot waktu belajar realistis yang menyesuaikan jam pulang sekolah dan kegiatan pribadi siswa.
3. **Material Predictor**: Membantu siswa mempersiapkan materi berikutnya sebelum diajarkan di kelas berdasarkan riwayat tugas dan kurikulum.

## 3. Konteks Pameran & Demo
Backend disiapkan untuk mendukung demo interaktif aplikasi mobile (APK) dan Bruno API Client pada pameran inkubasi. Akun demo standar disediakan melalui database seeder (`go run . seed --demo`):
- Siswa Demo (profil lengkap untuk pameran): `naufal@student.id` / `password123`
- Admin & data sintetis (`go run . seed`): `admin@stoodify.seed`, `guru001`–`guru003@stoodify.seed`, `user001...@stoodify.seed` — semua `rahasia123`
