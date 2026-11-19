# Stoodify AI Contract Specification

Dokumen ini kontrak resmi pertukaran data antara **Stoodify Backend (Go)** dan **service AI** (`stoodify-ai-1`, FastAPI). Semua nama field di bawah adalah yang **benar-benar dikirim dan dibaca** oleh `controllers/ai_controllers.go`. Kalau service AI mengubah bentuk respons, ubah dokumen ini di commit yang sama.

Prinsip kompatibilitas: backend menerima **lebih dari satu bentuk** respons supaya service AI boleh memakai penamaan gaya kontrak maupun gaya internal tanpa memecah integrasi. Bentuk yang diterima didaftar eksplisit per endpoint.

---

## 1. AI Scheduler (Rekomendasi Jadwal Belajar)

### Endpoint backend
`POST /api/ai/schedule-recommendations/generate` (auth Bearer, body opsional)

### Request dari client ke backend
```json
{ "date": "2026-11-13" }
```
- `date` opsional (`YYYY-MM-DD`), default hari ini.
- Body kosong (`{}`) ditoleransi: backend menganggap tanggal hari ini, tanpa error EOF.
- Format tanggal salah → 422 dengan `errors.date`.

### Payload backend → service AI
```json
{
  "user_id": 12,
  "date": "2026-11-13",
  "now": "2026-11-13T19:04:00+07:00",
  "timezone": "Asia/Jakarta",
  "preferences": {
    "start_time": "16:00",
    "end_time": "21:00",
    "session_duration": 60,
    "break_duration": 15,
    "max_sessions_per_day": 3,
    "active_days": "Monday,Tuesday,Wednesday,Thursday,Friday"
  },
  "tasks": [ /* model Task lengkap: id, title, material_name, deadline, difficulty, est_duration, type, status, progress, priority_score */ ],
  "schedules": [ /* jadwal sekolah: day_of_week, start_time, end_time, room, teacher */ ],
  "activities": [ /* aktivitas rutin siswa */ ],
  "existing_sessions": [
    { "session_id": 91, "task_id": 12, "session_date": "2026-11-18", "start_time": "16:00",
      "duration": 60, "status": "planned", "locked": true }
  ]
}
```
Catatan:
- `tasks` dikirim terurut `priority_score DESC, deadline ASC`, hanya yang `status <> 'done'`, dan **sudah menyertakan objek `subject`** (nama mapel).
- `schedules` dikirim dengan `title` (nama mapel) + `day_of_week`/`start_time`/`end_time`.
- `activities` menyertakan `day_of_week` untuk kegiatan mingguan.
- `locked: true` pada `existing_sessions` berarti slot itu **tidak boleh digeser/dipakai ulang**: sesi milik user (`ai_generated=false`), sesi yang mulai < 24 jam, atau status selain `planned`. Sampai engine mendukungnya, backend menyaring sendiri slot yang bentrok.
- `timezone` mengikuti env `TZ` server (default `Asia/Jakarta`).
- `break_duration`/`max_sessions_per_day` masih dikirim, tetapi **belum dipakai** engine (lihat `docs/divisi/AI-TODO.md`).
- Backend memanggil endpoint ini **otomatis** setiap ada perubahan tugas/konteks (debounce 3 detik, single-flight per user), jadi endpoint harus aman dipanggil berulang.

### Respons service AI → backend
Diterima dalam tiga bentuk, isi slot sama:

```json
{ "sessions": [ { "task_id": 2175, "recommended_date": "2026-11-13", "start_time": "16:30", "end_time": "17:30", "duration": 60, "target_progress": 30, "priority": "high", "reason": "Deadline besok dan kesulitan 5" } ] }
```

```json
{ "data": [ /* array slot yang sama */ ] }
```

```json
[ /* array slot polos, tanpa pembungkus */ ]
```

Alias field yang diterima per slot:

| Field dibaca backend | Alias yang juga diterima | Wajib |
|---|---|---|
| `task_id` | — | ya (harus milik user; kalau bukan → slot dibuang) |
| `recommended_date` | `session_date` | tidak; kosong = pakai tanggal target |
| `start_time` | — | ya, format `HH:MM` (tidak valid → slot dibuang) |
| `end_time` | — | tidak; kosong dihitung dari `start_time + duration` |
| `duration` | — | tidak; kosong/<=0 = `session_duration` preferensi |
| `target_progress` | — | tidak |
| `priority` | — | tidak (`high`/`medium`/`low`, hanya label) |
| `reason` | — | tidak |
| `task_title` | — | diterima tapi diabaikan (judul diambil dari DB) |

Aturan penanganan:
- **Pembersihan Sesi Planned Lama:** Sebelum memasukkan sesi rekomendasi baru, backend secara otomatis menghapus sesi-sesi berstatus `planned` lama milik user (`status = 'planned'`) agar tidak terjadi penumpukan/duplikasi slot belajar usang di kalender (ADR-006).
- Tidak ada slot valid (bukan milik user / jam rusak) → `502 "AI tidak memberikan slot yang bisa dipakai"`, tidak ada sesi dibuat.
- Sesi yang dibuat ditandai `ai_generated = true`, status `planned`.
- `message` respons tidak lagi menyebut sumber (dulu ada sufiks `(ai)` / `(fallback rule-based)`); backend tidak punya jalur rekomendasi lokal.

---

## 2. AI Material Predictor (Prediksi Materi Selanjutnya)

### Endpoint backend
`POST /api/ai/material-predictions/generate` (auth Bearer)

### Request dari client ke backend
```json
{ "subject_id": 33 }
```
`subject_id` wajib dan harus milik user (kalau tidak → 404).

### Payload backend → service AI
```json
{
  "subject_id": 33,
  "subject": "Matematika",
  "difficulty": 5,
  "class_level": "XII",
  "curriculum": "Kurikulum Merdeka",
  "history": [ /* maksimal 10 task terakhir mapel ini, urut deadline DESC; termasuk material_name */ ]
}
```

### Respons service AI → backend
Diterima sebagai objek langsung maupun dibungkus `{"data": {...}}`:

```json
{
  "predicted_topic": "Integral Tentu dan Luas Daerah",
  "confidence_score": 0.88,
  "summary": "Setelah integral substitusi dan parsial, materi berikutnya adalah penerapan integral tentu.",
  "prerequisites": ["Turunan Dasar", "Integral Substitusi"],
  "key_concepts": ["Teorema Dasar Kalkulus", "Batas Integrasi"],
  "suggested_activities": ["Pelajari grafik fungsi kuadrat", "Latihan substitusi batas integral"]
}
```

Alias dan normalisasi field:

| Dibaca backend | Alias | Normalisasi |
|---|---|---|
| `predicted_topic` | — | wajib; kosong = respons dianggap gagal → `502` |
| `confidence` | `confidence_score` | nilai 0..1 dikali 100 otomatis (kolom DB 0..100) |
| `reason` | `summary` | kalau dua-duanya kosong, diisi `"Konsep kunci: ..."` dari `key_concepts` |
| `prerequisites` | — | boleh array `["a","b"]` (digabung `", "`) atau string `"a, b"` |
| `preparation_suggestion` | — | kalau kosong, digabung dari `suggested_activities` dengan pemisah `; ` |
| `key_concepts` | — | hanya dipakai kalau `reason` dan `summary` kosong |

Kolom `material_predictions` yang disimpan: `predicted_material`, `confidence`, `reason`, `prerequisites`, `preparation_suggestion`, plus `user_id`/`subject_id`.

---

## 3. Penanganan Error, Timeout & Rate Limit

**Pembagian tanggung jawab:** backend Go **tidak punya fallback rule-based** dan tidak membuat jadwal/prediksi sendiri. Semua keputusan saat LLM bermasalah ada di service Python. Go hanya meneruskan hasilnya ke mobile.

### Yang harus dibalas service AI

| Situasi | Status dari Python | Body | Efek di backend |
|---|---|---|---|
| Normal | `200` | `{"sessions":[...]}` / `{"data":[...]}` / `[...]` untuk scheduler; objek/`{"data":{...}}` untuk predictor | disimpan, klien dapat `201` |
| LLM kena rate limit tapi Python punya rencana degradasi/cache | `200` | skema normal (boleh ditandai field tambahan, mis. `"degraded": true`) | disimpan apa adanya; backend tidak mengubahnya |
| LLM limit & tidak ada rencana | `429` | `{"message":"..."}` atau `{"detail":"..."}` + header `Retry-After` (detik) | **diteruskan**: klien dapat `429`, pesan & `Retry-After` ikut |
| Model/service sedang tidak siap | `503` | pesan + opsional `Retry-After` | **diteruskan** sebagai `503` |
| Error lain di Python | `4xx`/`5xx` selain 429/503 | bebas | klien dapat `502` generik; detail hanya masuk log server |
| Python tidak bisa dihubungi / timeout | — | — | klien dapat `502 "layanan AI tidak dapat dihubungi, silakan coba lagi"` |

Aturan tambahan:
- **Timeout** backend: default **15 detik**, diubah lewat env `AI_TIMEOUT_SECONDS`. Lewat dari batas ini request dibatalkan dan klien dapat `502`.
- **Tidak ada sesi/prediksi parsial**: kalau AI gagal, tidak ada baris DB yang ditulis.
- **Detail internal tidak dibocorkan**: untuk status selain 429/503, pesan Python tidak diteruskan ke klien (hanya ke log).
- **Payload `{}`** pada generate jadwal tetap ditoleransi tanpa error EOF.
- **Konfigurasi wajib**: `AI_SCHEDULER_URL` dan `AI_PREDICTOR_URL` harus terisi; kalau kosong, perintah `serve` menolak start (fail-fast saat boot, bukan 502 per request).
- `AI_API_KEY` opsional, dikirim sebagai `Authorization: Bearer <key>` bila diisi.

---

## 4. Umpan Balik (Feedback)

Saat user menekan akurat/tidak akurat pada sebuah prediksi, backend meneruskan ke engine:

```
POST {base AI_PREDICTOR_URL}/feedback
{ "data_feedback": [ { "prediction_id": 12, "subject_id": 33,
    "predicted_topic": "Integral Tentu", "confidence": 88,
    "reason": "...", "feedback": "accurate" } ] }
```

- Best-effort: timeout 3 detik, kegagalan hanya dicatat di log (tidak menggagalkan request user).
- Base URL diturunkan dari `AI_PREDICTOR_URL` (`.../predict` → `.../feedback`).

---

## 5. Yang Bukan Bagian dari Bridge AI

- `POST /api/ai/reschedule` **tidak memanggil service AI**. Endpoint ini murni lokal: mengambil sesi `planned` milik user lalu menyusun ulang jamnya berurutan dari `preferences.start_time` hari ini. Rescheduling adaptif berbasis AI belum diimplementasi.
- `GET /api/ai/schedule-recommendations`, `POST .../accept`, `POST .../reject`, `GET /api/ai/material-predictions/:subjectId`, `POST .../:id/feedback` hanya operasi database.
- **Priority Score** (`internal/priority`) dihitung lokal dengan formula PRD (urgensi 40%, kesulitan 20%, durasi 15%, jenis tugas 15%, risiko terlambat 10%) — bukan keluaran AI.
