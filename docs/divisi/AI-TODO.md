# TODO Divisi AI (`stoodify-ai-1`) — Penjadwalan Otomatis

Scope: service `stoodify-ai-1` (FastAPI). Backend tidak mengerjakan item di sini.
Dokumen ini disimpan di repo backend (repo AI bersifat **read-only** dari sisi kita); sampaikan isinya ke pengembang AI.
Konteks: backend akan memanggil `/schedule` **otomatis** setiap ada perubahan tugas (debounce + single-flight), tanpa alur terima/tolak. Kontrak backend: `stoodify-backend/docs/PLAN-AUTO-SCHEDULE.md`.

---

## 1. Wajib untuk MVP auto-schedule

- [ ] **Dukung `locked` pada `existing_sessions`.** Backend akan mengirim sesi yang tidak boleh digeser dengan penanda:
      ```json
      { "session_id": 91, "task_id": 12, "session_date": "2026-11-18", "start_time": "16:00",
        "duration": 60, "status": "planned", "locked": true }
      ```
      Arti: slot itu sudah milik user / terlalu dekat untuk digeser. Engine harus menganggapnya sebagai blok terpakai (seperti jadwal sekolah) dan tidak menaruh sesi baru di atasnya.
      Sampai ini ada, backend menyaring hasilnya sendiri — tapi lebih baik ditangani di engine.
      Lokasi: `adapter.py` (parsing `existing_sessions`, sekitar baris 194-209).
- [ ] **Latensi `/schedule` < 5 detik (p95).** Backend timeout 15 detik, tapi karena dipanggil otomatis, respon lambat membuat jadwal user telat berubah. Usul: fallback internal aktif bila LLM > 6 detik.
- [ ] **Pastikan `source`, `warnings`, `fallback_reason` selalu ada** di respons `/schedule` (sudah ada di `service.py:77-84`) dan `source`, `syllabus_reference` di `/predict` (`:110-119`). Backend sekarang menyimpannya; jangan dihapus/di-rename tanpa memberi tahu backend.

## 2. Keputusan kontrak yang perlu disepakati

- [ ] **`break_duration` & `max_sessions_per_day`**: backend mengirim keduanya, tapi `adapter.py:127-128` mengabaikannya. Pilih salah satu:
      - (a) engine memakainya (rekomendasi: hormati `max_sessions_per_day` saat menaruh slot, dan `break_duration` sebagai jeda minimum antar sesi), atau
      - (b) backend berhenti mengirim dan dokumen kontrak diperbaiki.
- [ ] **`class_level` & `curriculum`** pada `/predict`: sudah diterima `PredictionPayload` dan dipakai `baca_silabus`. Backend akan mengirim `class_level` hasil mapping `grade` (10→`X`, 11→`XI`, 12→`XII`, 13→`XIII`) dan `curriculum` default `"Kurikulum Merdeka"`. Konfirmasi format ini yang diinginkan.
- [ ] **`activities[].day_of_week`**: backend akan mengirim `Monday..Sunday` (dinormalisasi). Engine sudah mendukung (`adapter.py:180`), konfirmasi tidak ada perubahan format.

## 3. Fase lanjutan (belum dikerjakan backend)

- [ ] **Parameter horizon** untuk menyusun lebih dari satu hari dalam satu panggilan, mis. `{"date":"2026-11-18","days":7}` atau balasan berisi beberapa tanggal. Sekarang `adapt_request` hanya memakai satu `date` (`adapter.py:109-113`). Tanpa ini, backend harus memanggil 7× untuk rencana seminggu (mahal).
- [ ] **Feedback ingestion yang benar-benar dipakai**: endpoint `POST /feedback` dan `/feedback/per-mapel` sudah ada (`service.py:122-131`). Backend akan mengirim `{data_feedback:[{prediction_id, subject_id, predicted_topic, confidence, feedback, ...}]}` setiap user menekan akurat/tidak akurat. Pastikan format itu diterima dan hasil analisisnya memengaruhi prompt berikutnya (atau minimal tersimpan untuk evaluasi).

## 4. Dokumentasi yang harus diperbarui (sekarang basi)

- [ ] `docs/INTEGRASI_ARSITEKTUR_AI.md:62` masih menyatakan backend menjalankan `fallbackSlots` setelah 15 detik — **sudah dihapus** (backend bridge-only, ADR-005).
- [ ] `docs/INTEGRASI_ARSITEKTUR_AI.md:66` menyatakan backend belum mengirim `schedules`, `activities`, `existing_sessions` — **sudah dikirim** sejak commit `b88bdd0`.
- [ ] `docs/INTEGRASI_ARSITEKTUR_AI.md:166,279` menyebut kriteria penerimaan pesan `rekomendasi sesi belajar dibuat (ai)` — sufiks `(ai)` sudah dihapus.
- [ ] Tambahkan bagian **"Kontrak v1"** ringkas: payload masuk, bentuk respons, status yang diharapkan (200 plan normal/degradasi, 429 limit tanpa plan, 503 tidak siap), plus penjelasan `locked`.
- [ ] Catat bahwa backend memanggil otomatis (bukan atas klik user), jadi endpoint harus aman dipanggil berulang.

## 5. Operasional

- [ ] `AI_TIMEOUT_SECONDS` di engine (sekarang 10) harus tetap **lebih kecil** dari timeout backend (15) agar engine yang memutus lebih dulu dan masih sempat mengembalikan fallback.
- [ ] `GET /health` dipakai backend untuk smoke test deploy; pastikan tetap tanpa auth dan cepat.
- [ ] Log sudah memuat `source`/`latency_ms` (`service.py:74-76,106-108`) — pertahankan formatnya agar mudah dikorelasikan dengan log backend.
- [ ] Idempotensi: engine stateless; jangan menyimpan state per user di memori (backend yang mengatur antrian).

## 6. Pengujian yang perlu ditambah

- [ ] Payload dengan `existing_sessions[].locked=true` → slot itu tidak dipakai ulang.
- [ ] Payload dengan `tasks[].subject` kosong (backend lama) → tetap jalan, ada warning.
- [ ] `/schedule` dipanggil 5× berurutan untuk user yang sama → tidak ada kebocoran state, hasil konsisten.
- [ ] Respons 429 dari provider LLM → engine tetap balas 200 dengan `source=fallback` (bukan 500).
- [ ] `/predict` dengan `class_level` kosong vs terisi → hasil berbeda sesuai silabus.
