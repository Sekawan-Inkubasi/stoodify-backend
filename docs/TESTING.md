# Stoodify Testing Guide

Panduan pengujian otomatis untuk modul backend Stoodify.

---

## 1. Menjalankan Test

Jalankan seluruh test suite dengan perintah:
```bash
go test -v ./...
```

Untuk menguji package tertentu dengan rincian detail:
```bash
go test -v ./internal/priority
```

Untuk memeriksa coverage:
```bash
go test -cover ./...
```

---

## 2. Target Pengujian Utama
- **Priority Scoring Formula (`internal/priority/priority_test.go`)**:
  - Memastikan tugas dengan deadline dekat memiliki urgensi tinggi (>70).
  - Memastikan bobot kesulitan (1 s.d. 5) dan durasi terhitung proporsional.
  - Memastikan tugas dengan progres 100% menghasilkan skor 0.
  - Memverifikasi konsistensi sub-skor pada `CalculateBreakdown`.

---

## 3. Aturan Menulis Unit Test Baru
1. Tempatkan file test berdampingan dengan kode yang diuji menggunakan suffix `_test.go`.
2. Gunakan table-driven tests untuk menguji berbagai variasi input dan edge cases.
3. Jangan gunakan mock yang terlalu rumit jika fungsi murni (*pure function*) dapat diuji langsung.
