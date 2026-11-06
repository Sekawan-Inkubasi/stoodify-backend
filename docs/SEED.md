# Seeder Data Uji & Akun Demonstrasi

Stoodify Backend menyediakan dua jenis seeder:
1. **Akun Demonstrasi Khusus (`--demo`)**: Menyiapkan akun riil siswa SMK Telkom Sidoarjo jurusan SIJA (`naufal@student.id`) lengkap dengan jadwal sekolah dan tugas kejuruan untuk demo live fitur AI.
2. **Data Sintetis Massal (`seed`)**: Mengisi database dengan puluhan user acak berdomain `@stoodify.seed` untuk stresstest, uji pagination, atau mobile development umum.

---

## 1. Akun Demonstrasi (`--demo`)

### Cara Menjalankan:
```bash
# Opsi 1: Hapus database sampai bersih lalu pasang akun demo (Recommended)
go run . migrate:fresh --demo
# atau pakai binary compile:
./stoodify migrate:fresh --demo

# Opsi 2: Hanya tanamkan/refresh akun demo tanpa menghapus data tabel lain
go run . seed --demo
# atau:
./stoodify seed --demo
```

### Rincian Akun Demo:
- **Nama Siswa**: `Muhammad Naufal Rafa Al As'ad`
- **Email**: `naufal@student.id`
- **Password**: `password123`
- **Profil Sekolah**: SMK Telkom Sidoarjo, Kelas 12, Jurusan Sistem Informasi, Jaringan, Aplikasi.
- **Preferensi Waktu AI**: Jam belajar mandiri 16:00 - 21:00 WIB, durasi per sesi 60 menit, jeda istirahat 15 menit, maksimal 3 sesi per hari.
- **Jadwal Pelajaran**: Senin s.d. Jumat (07:00 - 15:10 WIB) sesuai jadwal riil kelas XII SIJA 2 (IoT, SKJ 2, KA 2, PM 3, PaaS, dll.).
- **Daftar Tugas Aktif**: 8 tugas produktif kejuruan dengan tanggal tenggat bervariasi (9 Nov - 30 Nov 2026) dan priority score yang terhitung otomatis.

---

## 2. Seeder Sintetis Massal (Uji Mobile Dev)

```bash
go run . seed --users=50 --tasks=40          # default: ~14 ribu baris
go run . seed --users=200 --tasks=100        # skala super besar (~140 ribu baris)
go run . seed --users=50 --tasks=40 --reset   # hapus data seed lama (@stoodify.seed), lalu isi ulang
go run . seed --quiet                       # tanpa log progres
```

### Akun Uji Sintetis:
| Role | Email | Password |
|---|---|---|
| Admin | `admin@stoodify.seed` | `rahasia123` |
| Guru | `guru001@stoodify.seed`–`guru003@stoodify.seed` | `rahasia123` |
| Siswa | `user001@stoodify.seed`–`user<NNN>@stoodify.seed` | `rahasia123` |

---

## 3. Catatan Teknis

- Password di-hash bcrypt **sekali** lalu dipakai semua user sintetis untuk menghemat waktu seeding (cost 14 lambat ~0,3 detik per hash).
- `priority_score` dihitung lewat formula priority engine PRD sungguhan, bukan angka acak.
- `--reset` hanya menghapus data seed berdomain `@stoodify.seed` dengan urutan anak → induk agar aman dari foreign key, dan **tidak menyentuh** akun non-seed.
- `migrate:fresh` melakukan `DROP` pada seluruh tabel dan membuat ulang skema bersih sebelum menjalankan seeder.