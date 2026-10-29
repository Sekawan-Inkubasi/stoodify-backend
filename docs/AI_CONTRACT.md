# Stoodify AI Contract Specification

Dokumen ini merupakan kontrak resmi pertukaran data (payload request/response) antara Stoodify Backend (Golang) dan modul / service kecerdasan buatan (AI) Stoodify.

---

## 1. AI Scheduler (Rekomendasi Jadwal Belajar)

### Endpoint Backend
`POST /api/ai/schedule-recommendations/generate`

### Request Payload (Client -> Backend -> AI)
```json
{
  "date": "2026-10-31" // Opsional, default tanggal hari ini (YYYY-MM-DD)
}
```

### Konteks Data yang Dikirim Backend ke Service AI
- **Tasks Aktif**: Daftar tugas siswa yang belum selesai (`status <> 'done'`), diurutkan berdasarkan `priority_score DESC, deadline ASC`.
- **Jadwal Pelajaran (`Schedules`)**: Jam masuk & pulang sekolah pada hari target.
- **Aktivitas Rutin (`Activities`)**: Jadwal kegiatan ekstrakurikuler / les / ibadah siswa.
- **Preferensi Belajar Siswa (`UserPreference`)**:
  - `start_time` (contoh: `"16:00"`)
  - `end_time` (contoh: `"21:00"`)
  - `session_duration` (menit per sesi, misal `45`)
  - `break_duration` (menit istirahat, default `15`)
  - `max_sessions_per_day` (kuota sesi harian, default `3`)

### Response Expected (AI -> Backend -> Client)
Backend mengharapkan array rekomendasi sesi belajar:
```json
{
  "success": true,
  "message": "rekomendasi jadwal belajar berhasil digenerate",
  "data": [
    {
      "task_id": 2175,
      "task_title": "Latihan Soal Integral Substitusi & Parsial",
      "session_date": "2026-10-31",
      "start_time": "16:30",
      "end_time": "17:30",
      "duration": 60,
      "target_progress": 30,
      "priority": "high",
      "reason": "Deadline besok sore dan tingkat kesulitan tinggi (5)"
    }
  ]
}
```

---

## 2. AI Material Predictor (Prediksi Materi Selanjutnya)

### Endpoint Backend
`POST /api/ai/material-predictions/generate`

### Request Payload
```json
{
  "subject_id": 332
}
```

### Konteks Data yang Disediakan Backend
- **Riwayat Tugas & Materi**: Daftar tugas lampau pada mapel tersebut beserta field `material_name` dan tanggal penugasan.
- **Data Siswa**: `grade` (tingkat kelas) dan `major` (jurusan, misal: SIJA, RPL, IPA).

### Response Expected
```json
{
  "success": true,
  "message": "prediksi materi berhasil dibuat",
  "data": {
    "subject_id": 332,
    "predicted_topic": "Integral Tentu dan Luas Daerah",
    "confidence_score": 0.88,
    "summary": "Setelah menyelesaikan teknik integral substitusi dan parsial, materi kurikulum selanjutnya adalah penerapan integral tentu untuk menghitung luas daerah di bawah kurva.",
    "prerequisites": ["Turunan Dasar", "Integral Substitusi"],
    "key_concepts": ["Teorema Dasar Kalkulus", "Batas Integrasi"],
    "suggested_activities": ["Pelajari grafik fungsi kuadrat", "Latihan substitusi batas integral"]
  }
}
```

---

## 3. Aturan Fallback & Toleransi
1. Jika service AI eksternal timeout (>5 detik) atau mengembalikan error, backend secara otomatis beralih ke algoritma heuristik rule-based internal.
2. Payload JSON kosong (`{}`) pada endpoint rekomendasi jadwal ditoleransi tanpa error EOF dan akan mengasumsikan tanggal hari ini.
