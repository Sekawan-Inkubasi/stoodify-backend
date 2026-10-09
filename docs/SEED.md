# Seeder Data Uji

Mengisi database dengan banyak data sintetis untuk uji Mobile Dev / demo.
Semua data memakai domain `@stoodify.seed` sehingga tidak tercampur akun asli
dan bisa dihapus bersih kapan saja.

## Menjalankan

```bash
go run ./seeder -users 50 -tasks 40          # default: ~14 ribu baris
go run ./seeder -users 200 -tasks 100        # skala super besar (~140 ribu baris)
go run ./seeder -users 50 -tasks 40 -reset   # hapus data seed lama, lalu isi ulang
go run ./seeder -quiet                       # tanpa log progres
```

Butuh koneksi DB sama seperti server (`.env` → `POSTGRES_URL`). Migrasi tabel
dijalankan otomatis saat boot seeder.

## Akun uji

| Role | Email | Password |
|---|---|---|
| Admin | `admin@stoodify.seed` | `admin123` |
| Guru | `guru001@stoodify.seed`–`guru003@stoodify.seed` | `rahasia123` |
| Siswa | `user001@stoodify.seed`–`user<NNN>@stoodify.seed` | `rahasia123` |

## Distribusi data per user siswa

- **Preferensi** 1:1, jam belajar 14:00–20:00 acak.
- **Mapel** 4–8, warna & kesulitan acak.
- **Jadwal** 4–6 hari sekolah, tanpa bentrok dalam sehari (07:00–16:00).
- **Tugas** `-tasks` per user, deadline menyebar: 30% sudah lewat, 15% hari ini,
  15% besok, 40% 2–14 hari ke depan — supaya status `late`, notifikasi H-1/H-0,
  upcoming/overdue, dan statistik terlihat hidup.
- **Subtugas** 0–5 per tugas.
- **Aktivitas** 5–8 per user.
- **Sesi belajar** 1–2 planned per tugas aktif (ai_generated), sesi completed
  untuk tugas selesai, sesi skipped untuk tugas telat.
- **Notifikasi** H-0 & H-1 untuk tugas belum selesai, warning untuk tugas dekat
  deadline, late untuk tugas telat.
- **Prediksi materi** 1–3 per mapel.

## Catatan

- Password di-hash bcrypt **sekali** lalu dipakai semua user — hemat waktu
  seeding (cost 14 lambat ~0,3 detik per hash).
- `priority_score` dihitung lewat priority engine sungguhan, bukan angka acak.
- `-reset` menghapus data seed dengan urutan anak → induk agar aman dari
  foreign key, dan **tidak menyentuh** akun non-seed.