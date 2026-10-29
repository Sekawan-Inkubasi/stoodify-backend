# Stoodify Security Policy & Hard Limits

Panduan batasan keamanan keras (*hard limits*) untuk developer dan AI Agent.

---
## 1. Larangan Mutlak (Hard Limits)
- **Dilarang Mengekspos Kredensial**: Jangan pernah mencatat (*log*) password mentah, token JWT, connection string database, atau secret key ke stdout atau file dokumentasi publik. Gunakan placeholder `[REDACTED]` dalam laporan.
- **Dilarang Bypass Multi-Tenant Isolation**: Setiap operasi database yang mengakses data pengguna wajib memfilter berdasarkan ID pengguna yang diautentikasi dari context JWT.
- **Dilarang Mengembalikan Raw SQL Error**: Selalu gunakan fungsi `response.ServerError` yang sudah disanitasi agar tidak mengekspos struktur tabel internal ke publik.

## 2. Spesifikasi Autentikasi & Password
- **Algoritma JWT**: HMAC-SHA256 (`HS256`).
- **Masa Berlaku Access Token**: 15 Menit.
- **Masa Berlaku Refresh Token**: 7 Hari (hanya hash token yang disimpan di tabel `refresh_tokens`).
- **Password Hashing**: Bcrypt dengan work factor (cost) minimal 14.
