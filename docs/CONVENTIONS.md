# Stoodify Coding & Git Conventions

Standar dan konvensi penulisan kode untuk memastikan konsistensi dan mencegah technical debt.

---

## 1. Aturan Penulisan Go
- Selalu jalankan `gofmt` sebelum menyimpan berkas.
- Gunakan camelCase untuk variabel lokal dan PascalCase untuk struct / method publik.
- Tangani error secara eksplisit; jangan abaikan error tanpa penjelasan logis.
- Dalam controller, gunakan context request:
  ```go
  ctrl.DB.WithContext(konteks.Request.Context())
  ```

## 2. Binding & Validasi Gin
- Gunakan tag `binding` pada DTO request:
  - Field wajib: `binding:"required"`
  - Field opsional: `binding:"omitempty"` (gunakan pointer jika membedakan nilai nol/kosong)
  - Validasi panjang string: `min=1,max=100`
  - Hindari typo validator seperti `requrired` (selalu gunakan kata kunci resmi Gin validator v10).

## 3. Format Response Standar
Setiap respons HTTP wajib dibungkus melalui package `internal/response`:
- `response.OK(c, "pesan sukses", data)`
- `response.OKWithMeta(c, "daftar data", list, meta)`
- `response.Created(c, "data dibuat", data)`
- `response.NoContent(c)`
- `response.NotFound(c, "pesan tidak ditemukan")`
- `response.ValidationError(c, err)`
- `response.ServerError(c, err)`

## 4. Git Commit Convention
Gunakan format [Conventional Commits](https://www.conventionalcommits.org/):
- `feat(...)`: Fitur baru
- `fix(...)`: Perbaikan bug
- `docs(...)`: Perubahan dokumentasi
- `test(...)`: Penambahan atau perbaikan unit test
- `refactor(...)`: Refactoring kode tanpa mengubah fungsionalitas
- `chore(...)`: Pemeliharaan build, dependensi, atau script
