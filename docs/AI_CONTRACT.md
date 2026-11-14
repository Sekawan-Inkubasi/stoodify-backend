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
  "existing_sessions": [ /* sesi planned pada tanggal target, agar AI tidak menumpuk slot */ ]
}
```
Catatan: `tasks` dikirim terurut `priority_score DESC, deadline ASC` dan hanya yang `status <> 'done'`. `break_duration`/`max_sessions_per_day` memakai default 15/3 bila preferensi siswa masih kosong.

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
- Tidak ada slot valid (bukan milik user / jam rusak) → `502 "AI tidak memberikan slot yang bisa dipakai"`, tidak ada sesi dibuat.
- Sesi yang dibuat ditandai `ai_generated = true`, status `planned`.
- `message` respons menyebut sumbernya: `(ai)` atau `(fallback rule-based)`.

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
  "grade": 12,
  "major": "Sistem Informasi Jaringan Aplikasi",
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
| `predicted_topic` | — | wajib; kosong = respons dianggap gagal → fallback |
| `confidence` | `confidence_score` | nilai 0..1 dikali 100 otomatis (kolom DB 0..100) |
| `reason` | `summary` | kalau dua-duanya kosong, diisi `"Konsep kunci: ..."` dari `key_concepts` |
| `prerequisites` | — | boleh array `["a","b"]` (digabung `", "`) atau string `"a, b"` |
| `preparation_suggestion` | — | kalau kosong, digabung dari `suggested_activities` dengan pemisah `; ` |
| `key_concepts` | — | hanya dipakai kalau `reason` dan `summary` kosong |

Kolom `material_predictions` yang disimpan: `predicted_material`, `confidence`, `reason`, `prerequisites`, `preparation_suggestion`, plus `user_id`/`subject_id`.

---

## 3. Aturan Fallback & Toleransi

1. **Timeout**: default **15 detik** (PRD §Non-Fungsional), dapat diubah lewat env `AI_TIMEOUT_SECONDS` (mis. `5`). Lewat dari batas ini, panggilan dibatalkan.
2. Fallback rule-based aktif kalau: URL AI kosong, request gagal/timeout, status non-2xx, body tidak bisa di-parse, atau hasil kosong/tidak ada slot valid.
3. Fallback bisa dimatikan dengan `AI_FALLBACK_ENABLED=false`; saat itu kegagalan AI menjadi `502 "layanan AI tidak dapat dihubungi, silakan coba lagi"`.
4. Payload `{}` pada generate jadwal ditoleransi tanpa error EOF.
5. **Batas fallback lokal**: `fallbackSlots` dan `fallbackPrediction` tidak mempertimbangkan `schedules`/`activities`/`existing_sessions` — hanya menjejalkan tugas terprioritas ke rentang preferensi. Service AI tetap sumber rekomendasi utama; fallback hanya jaring pengaman demo.

---

## 4. Yang Bukan Bagian dari Bridge AI

- `POST /api/ai/reschedule` **tidak memanggil service AI**. Endpoint ini murni lokal: mengambil sesi `planned` milik user lalu menyusun ulang jamnya berurutan dari `preferences.start_time` hari ini. Rescheduling adaptif berbasis AI belum diimplementasi.
- `GET /api/ai/schedule-recommendations`, `POST .../accept`, `POST .../reject`, `GET /api/ai/material-predictions/:subjectId`, `POST .../:id/feedback` hanya operasi database.
- **Priority Score** (`internal/priority`) dihitung lokal dengan formula PRD (urgensi 40%, kesulitan 20%, durasi 15%, jenis tugas 15%, risiko terlambat 10%) — bukan keluaran AI.
