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
