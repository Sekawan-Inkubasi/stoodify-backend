# Stoodify Backend Agent Guidelines & Master Map

Selamat datang AI Agent. File ini adalah **peta navigasi utama dan aturan main** dalam berkontribusi pada repository `stoodify-backend`. Ikuti panduan ini agar tidak melanggar kontrak global atau merusak arsitektur yang sudah ada.

---

## 1. Peta File Dokumentasi Khusus Agen

Sebelum melakukan perubahan atau membaca kode, baca file-file referensi berikut sesuai kebutuhan task:

### 🏛️ Arsitektur & Keputusan Desain
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) — Diagram alur sistem, domain boundaries, pola kepemilikan data (*multi-tenant user isolation*), dan integrasi AI.
- [`docs/DECISIONS.md`](DECISIONS.md) — Architecture Decision Records (ADR) penjelasan mengapa keputusan teknis diambil (e.g. komputasi dinamis `priority_breakdown`, format OpenCollection Bruno YAML, dll).
- [`docs/STACK.md`](STACK.md) — Tech stack detail, versi Go/PostgreSQL/Gin, tata letak folder, dan dependency limits.

### 📐 Konvensi & Standar Kode
- [`docs/CONVENTIONS.md`](CONVENTIONS.md) — Aturan penulisan kode Go, validasi tag Gin binding, format envelope response JSON, dan git commit convention.
- [`docs/TESTING.md`](TESTING.md) — Standar pengujian Go (`go test`), mocking DB/time, dan target pengujian logika inti (priority engine).
- [`docs/ERRORS.md`](ERRORS.md) — Daftar kegagalan umum (*known pitfalls*), bug yang pernah diatasi, dan cara troubleshooting yang tepat.

### 📊 Status & Manajemen Tugas
- [`docs/to-do.md`](to-do.md) — Single source of truth backlog fitur, bug fix, dan integrasi mobile (Tahap 1 s.d. Tahap 4).
- [`docs/STATE.md`](STATE.md) — Catatan status aktif dan serah terima antar sesi agent.
- [`docs/CONTEXT.md`](CONTEXT.md) — Konteks bisnis Stoodify, target pengguna (siswa SMP/SMA/SMK Indonesia), dan jadwal pameran inkubasi.

### 📖 Pengetahuan & Spesifikasi API
- [`README.md`](../README.md) — Panduan setup lokal dan ringkasan fitur.
- [`docs/API.md`](API.md) — Katalog 61 endpoint HTTP backend lengkap dengan contoh request dan response.
- [`docs/AI_CONTRACT.md`](AI_CONTRACT.md) — Kontrak spesifikasi payload JSON antara backend Go dan AI Specialist (Scheduler & Predictor).
- [`docs/SECURITY.md`](SECURITY.md) — Batasan keras keamanan, sanitasi database error, hashing password, dan penanganan token JWT.
- [`docs/GLOSSARY.md`](GLOSSARY.md) — Kamus istilah domain (Priority Score, Study Session, Heuristic Fallback, Day of Week, dll).

---

## 2. Aturan Emas Agen (Hard Rules)

1. **Data Ownership Filter**: Setiap query SELECT, UPDATE, DELETE wajib menyertakan `user_id = ?` dari klaim token `currentUser(c).ID`. Jangan pernah mengekspos data milik siswa lain!
2. **Envelope Response Konsisten**: Selalu gunakan package `stoodify_backend/internal/response` (`response.OK`, `response.Created`, `response.Fail`, `response.ValidationError`, `response.ServerError`).
3. **Sanitasi Error**: Jangan pernah mengirim `err.Error()` mentah dari SQL/database ke client dalam response 500 (`response.ServerError` sudah otomatis menyembunyikan raw error).
4. **Verifikasi Build**: Setelah melakukan perubahan kode Go, selalu jalankan:
   ```bash
   go test ./... && go vet ./... && go build ./...
   ```
5. **Format Bruno Collection**: Koleksi API client Bruno wajib menggunakan format **OpenCollection specification v1.0.0 berformat `.yml`** (bukan `.bru`).
