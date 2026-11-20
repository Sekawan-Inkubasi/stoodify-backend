# Known Errors & Troubleshooting (Pitfalls)

Daftar masalah yang pernah terjadi dan solusi terverifikasi agar agen tidak mengulang kesalahan yang sama.

---

## 1. Token JWT 'Token Tidak Valid atau Sudah Kedaluwarsa'
- **Gejala**: Request mengembalikan status `401 Unauthorized`.
- **Penyebab**: Access token hanya berdurasi 15 menit. Atau terdapat perbedaan timestamp sistem (misal jam di container/host berbeda dengan `iat`/`exp` token).
- **Solusi**: Lakukan refresh token melalui `POST /api/auth/refresh` atau login ulang `POST /api/auth/login`. Pastikan `JWT_SECRET` pada `.env` konsisten antara environment yang membuat dan memvalidasi token.

## 2. Bruno UI Folder Kosong Saat Menggunakan File `.bru`
- **Gejala**: Di aplikasi Bruno versi 4, folder koleksi terlihat kosong atau request tidak dapat dieksekusi.
- **Penyebab**: Bruno versi 4.x menggunakan spesifikasi **OpenCollection v1.0.0 (`.yml`)** bukan format `.bru`.
- **Solusi**: Jangan membuat file `.bru`. Seluruh request API didefinisikan dalam file `.yml` dengan manifest `opencollection.yml`.

## 3. Error EOF Saat Generate Rekomendasi Jadwal AI
- **Gejala**: Panggilan `POST /api/ai/schedule-recommendations/generate` gagal dengan status 400 ketika body request kosong.
- **Penyebab**: `ShouldBindJSON` gagal karena body HTTP kosong.
- **Solusi**: Controller telah disesuaikan untuk mengabaikan error `EOF` dan menganggap tanggal target adalah hari ini secara default.

## 4. Validasi Grade Memblokir Siswa SMP
- **Gejala**: Siswa kelas 7, 8, atau 9 gagal mendaftar dengan pesan validation error.
- **Penyebab**: Tag validasi sebelumnya adalah `min=10`.
- **Solusi**: Sudah dilonggarkan menjadi `min=7,max=13` untuk mengakomodasi siswa jenjang SMP, SMA, dan SMK 4 tahun.

## 5. Pesan Error Validasi Membocorkan Nama Struct Go

**Gejala:** client menerima pesan seperti
`Key: 'RegisterInput.School' Error:Field validation for 'School' failed on the 'required' tag`.

**Penyebab:** `response.ValidationError` mengirim `err.Error()` mentah dari validator.

**Perilaku sekarang:**

| Environment | `message` | `errors` |
| --- | --- | --- |
| development (default) | "Body request tidak valid, harap isi formulir dengan benar" | pesan per field **+** `body` berisi error mentah untuk debugging |
| production (`APP_ENV=production`) | "Data yang dikirim belum lengkap atau tidak valid" | **hanya** pesan per field bahasa Indonesia; tanpa nama struct/tag/path |

Contoh production:

```json
{
  "success": false,
  "message": "Data yang dikirim belum lengkap atau tidak valid",
  "errors": { "school": "wajib diisi", "grade": "wajib diisi", "major": "wajib diisi" }
}
```

Nama field mengikuti tag `json` (snake_case), didaftarkan sekali di `routes.SetupRoutes`
lewat `response.DaftarkanNamaFieldJSON`. Jangan menambahkan `err.Error()` mentah ke response
baru — selalu pakai `response.ValidationError` / `response.ServerError`.

**Catatan penting:** `docker-compose.yml` menetapkan `APP_ENV=production`, jadi container
produksi selalu menyembunyikan detail internal. Untuk melihat detail saat debugging di
lokal, jalankan `go run . serve` dengan `APP_ENV=development` (default).
