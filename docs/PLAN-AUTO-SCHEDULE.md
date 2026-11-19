# Plan: Penjadwalan Otomatis (Auto-Schedule)

Dokumen perencanaan perubahan **backend** untuk fitur rekomendasi jadwal yang berjalan otomatis.
Ruang lingkup: `stoodify-backend` saja. Perubahan divisi lain ada di:

- Mobile: [`docs/divisi/MOBILE-TODO.md`](divisi/MOBILE-TODO.md)
- AI (`stoodify-ai-1`): `docs/TODO-AUTO-SCHEDULE.md` di repo AI

Status: **rencana, belum diimplementasikan.**

---

## 1. Tujuan

1. Setiap perubahan tugas (dan konteks terkait) **otomatis** memicu penyusunan ulang jadwal belajar.
2. Hasilnya **langsung dipakai** — tidak ada alur terima/tolak, tidak ada popup konfirmasi.
3. Jadwal terasa **milik user**: sesi yang dia ubah sendiri tidak pernah digeser otomatis.
4. Bila AI gagal/limit, jadwal lama **tetap utuh**; user tidak melihat error.
5. Backend tetap **bridge murni**: tidak ada logika penjadwalan di Go. Semua penentuan slot oleh service AI.

---

## 2. Prinsip

| Wilayah | Pemilik |
|---|---|
| Menentukan tugas mana di slot jam berapa | **AI (Python)** |
| Kapan AI dipanggil, data apa yang dikirim, hasil disimpan bagaimana | **Backend (Go)** |
| Sesi mana yang tidak boleh digeser (selesai/pinned) | **Backend** (integritas data) |
| Normal vs fallback | **AI**, backend hanya meneruskan `source` |

Backend **tidak boleh** punya rumus penjadwalan sendiri (fallback rule-based sudah dihapus di ADR-005 dan tidak dikembalikan).

---

## 3. Kondisi saat ini (baseline)

| Aspek | Kenyataan sekarang | Bukti |
|---|---|---|
| Pemicu | Hanya manual dari mobile | `routes/routes.go:273` satu-satunya pemanggil `GenerateSchedule` |
| Trigger dari tugas | Tidak ada | `controllers/task_controllers.go` tidak menyentuh AI |
| Alur persetujuan | Masih ada accept/reject | `routes.go:275-276`, `ai_controllers.go` |
| Idempotensi | Sudah: hapus sesi AI `planned` di tanggal itu sebelum insert | `ai_controllers.go:170-179` |
| Sesi manual | Sudah aman: hanya `ai_generated=true` yang dihapus | idem |
| Metadata AI | `source`/`warnings`/`fallback_reason` dibuang | `parseSchedulerResponse` hanya ambil slot |
| Konteks ke AI | `tasks` dikirim **tanpa** `subject` (tidak di-preload) | `ai_controllers.go:77-81` |
| Predictor | Kirim `grade`/`major`; AI mengharapkan `class_level`/`curriculum` | `service.py:41-46`, `predictor.py:21` |
| Activity | Tidak punya hari → kegiatan `weekly` dianggap tiap hari | `migrations.go` (Activity), `adapter.py:180-192` |
| Rate limit manual | Tidak ada | — |
| Undo | Tidak ada | — |

---

## 4. Rancangan perubahan

### 4.1 Data model (migrasi additive, aman untuk data lama)

| Tabel | Kolom baru | Fungsi |
|---|---|---|
| `activities` | `day_of_week varchar(10)` | kegiatan mingguan punya hari; kosong = perilaku lama |
| `study_sessions` | `ai_source varchar(20)` | `ai` / `fallback` dari engine |
| `study_sessions` | `ai_warnings text` | peringatan engine, ditampilkan/di-log |
| `study_sessions` | `plan_version int` | penanda batch hasil generate (untuk undo) |
| `material_predictions` | `ai_source varchar(20)` | idem untuk prediksi |
| `material_predictions` | `syllabus_reference text` | referensi silabus dari engine |
| **baru** `schedule_snapshots` | `id, user_id, session_date, payload jsonb, created_at` | menyimpan jadwal sebelum regenerasi untuk fitur "Urungkan" |

**Semantik pin:** `ai_generated = true` → boleh digeser otomatis. `ai_generated = false` → milik user (dibuat manual atau diedit), **tidak pernah** disentuh regenerasi. Tidak perlu kolom baru.

### 4.2 Planner worker (`workers/planner.go`, baru)

- Struct `Planner` dengan channel `permintaan chan uint` (userID) + `map[uint]*state`.
- Per user: `dirty bool`, `jalan bool`, `lastRun time.Time`.
- `Enqueue(userID)`:
  - set `dirty = true`
  - jika `jalan` → return (nanti dijalankan sekali lagi setelah selesai)
  - jika `time.Since(lastRun) < debounce (3s)` → jadwalkan lewat `time.AfterFunc`, bukan langsung
  - selain itu → jalankan
- Saat selesai: `lastRun = now`, `jalan = false`, dan bila `dirty` masih true → enqueue sekali lagi.
- Dijalankan dari `runServe` sebagai goroutine (pola sama dengan `workers.JalankanScheduler`), dihentikan lewat channel yang sama.

Efek: 4 tugas dalam 30 detik → **1 panggilan** (plus maksimal 1 susulan), bukan 4.

### 4.3 Trigger hook (controller → planner)

Panggil `planner.Enqueue(user.ID)` **setelah commit DB berhasil** pada:

| Aksi | File |
|---|---|
| Task create / update / delete / update status / update progress | `controllers/task_controllers.go` |
| Subtask create / update / delete | `controllers/subtask_controllers.go` |
| Schedule create / update / delete | `controllers/schedule_controllers.go` |
| Activity create / update / delete | `controllers/activity_controllers.go` |
| Preference update | `controllers/user_controllers.go` |
| Subject delete (menyentuh tugas) | `controllers/subject_controllers.go` |

Hook bersifat *fire-and-forget*: tidak menambah latency respons, tidak pernah menggagalkan request user.

### 4.4 Context builder (refactor, bukan logika baru)

Ekstrak isi `GenerateSchedule` menjadi fungsi yang bisa dipakai dua jalur (manual & planner):

```go
type jadwalContext struct {
    UserID           uint
    Tanggal          time.Time
    Tasks            []migrations.Task   // Preload("Subject")
    Pref             migrations.UserPreference
    Schedules        []migrations.Schedule // Preload("Subject")
    Activities       []migrations.Activity
    ExistingSessions []migrations.StudySession
}

func (ctrl *AIController) bangunKonteks(ctx context.Context, user migrations.User, tanggal time.Time) (jadwalContext, error)
```

Aturan isi `ExistingSessions`:
- status `planned`
- `session_date >= tanggal`
- **semua** sesi dikirim (termasuk milik user) supaya AI tidak menumpuk, tetapi sesi pinned/frozen ditandai agar tidak digeser:

```json
{ "session_id": 91, "task_id": 12, "session_date": "2026-11-18", "start_time": "16:00",
  "duration": 60, "status": "planned", "locked": true }
```

`locked: true` untuk sesi `ai_generated=false`, sesi yang mulai < 24 jam, atau status `in_progress`/`completed`.
> Butuh tambahan dukungan `locked` di AI (lihat TODO AI). Sampai itu ada, backend **menyaring** sesi locked dari hasil AI (tidak menghapus/menggeser barisnya) — jadi tetap aman.

### 4.5 Persister (transaksi)

Satu transaksi per regenerasi:

1. Simpan snapshot jadwal saat ini ke `schedule_snapshots` (untuk undo, retensi 7 hari).
2. Hapus `study_sessions` dengan: `user_id`, `session_date = tanggal`, `ai_generated = true`, `status = planned`, **bukan** pinned/frozen.
3. Insert sesi baru dengan `ai_source`/`ai_warnings` dari metadata AI.
4. Commit. Kalau AI gagal → **tidak ada perubahan apa pun**.

### 4.6 Endpoint manual + rate limit

`POST /api/ai/schedule-recommendations/generate` tetap ada, tapi jadi **manual override**:

- Limit: 1×/10 menit per user (`429` + `Retry-After`), plus bypass untuk tugas urgent tidak berlaku di jalur manual.
- Pesan (envelope `message`):
  - terlalu cepat: `"Jadwal kamu baru saja diperbarui. Coba lagi dalam {N} menit, atau geser sendiri sesi yang mau diubah."`
  - tidak ada perubahan: `"Jadwal kamu sudah paling pas — belum ada yang perlu disusun ulang."` (200)
  - AI sibuk: `"AI sedang sibuk. Jadwal lama tetap dipakai, kami coba lagi otomatis."` (429/503 pass-through)
- Respons tetap `201` + daftar sesi (kompatibel dengan mobile sekarang).

### 4.7 Deprecation accept/reject

- Endpoint `POST /api/ai/schedule-recommendations/:id/accept|reject` **tetap ada** (mobile lama tidak error) tetapi ditandai *deprecated* di `docs/API.md` & `openapi.yaml`.
- Regenerasi otomatis tidak pernah membuat sesi berstatus "menunggu persetujuan"; semua langsung `planned`.
- `reject` tetap berguna sebagai "lewati sesi ini" dan hasilnya `skipped`.

### 4.8 Rollover harian (worker)

Tambahan di `workers/scheduler.go` (ticker 1 menit yang sudah ada):
- Setiap hari pada 00:05 lokal: `planner.Enqueue(userID)` untuk semua user aktif (`last_login`/`updated_at` 30 hari terakhir, maksimal N user per tick).
- Tujuan: besok sudah punya jadwal walau app belum dibuka.

### 4.9 Undo

- `POST /api/ai/schedule-recommendations/undo` (auth) → ambil snapshot terbaru untuk hari itu, hapus sesi AI saat ini, pulihkan snapshot.
- Snapshot kedaluwarsa > 7 hari dibersihkan worker.
- Mobile memakai ini untuk tombol **Urungkan** pada toast.

### 4.10 Perbaikan kontrak AI (bagian dari pekerjaan ini)

| Perbaikan | Detail |
|---|---|
| `Preload("Subject")` pada tasks | agar `subject.name` terkirim ke AI |
| `Preload("Subject")` pada schedules | agar blok sekolah punya `title` |
| Predictor kirim `class_level` + `curriculum` | `grade` 10/11/12/13 → `X/XI/XII/XIII`; `curriculum` dari env `AI_CURRICULUM` (default `Kurikulum Merdeka`) |
| `timezone` dari env `TZ` | bukan hardcode |
| Simpan `source`/`warnings`/`fallback_reason` | kolom 4.1 + log |
| Forward feedback | `POST /api/ai/material-predictions/:id/feedback` meneruskan ke `POST /feedback` ai-1 (best-effort, timeout 3s, gagal = log) |
| `break_duration` & `max_sessions_per_day` | tetap dikirim, didokumentasikan sebagai **belum dipakai engine** (atau dihapus setelah AI menambah dukungan) |

### 4.11 Observability

- Log terstruktur: `planner: user=%d tanggal=%s sesi=%d source=%s ms=%d`.
- Log warning engine (`ai_warnings`) saat ada.
- Hitungan sederhana (bisa dipromosikan ke metrik nanti): jumlah regenerate, jumlah gagal, jumlah undo.

---

## 5. Perubahan API (ringkas)

| Endpoint | Perubahan |
|---|---|
| `POST /api/ai/schedule-recommendations/generate` | jadi manual override + rate limit 1×/10 menit + pesan baru |
| `POST /api/ai/schedule-recommendations/undo` | **baru** — kembalikan jadwal ke snapshot terakhir |
| `PATCH /api/study-sessions/:id` | menandai sesi `ai_generated=false` saat user mengubah jam/tanggal (pin) |
| `POST/PATCH /api/activities` | menerima `day_of_week` |
| `GET /api/study-sessions/` | response menyertakan `ai_source` (opsional, tidak breaking) |
| accept/reject | tetap, ditandai deprecated |
| Response generate | tambah field `ai_source`, `ai_warnings` per sesi |

---

## 6. Aturan perilaku (kontrak internal)

| Aturan | Nilai |
|---|---|
| Debounce per user | 3 detik |
| Single-flight | 1 regenerasi berjalan per user; trigger berikutnya jadi `dirty` |
| Cap manual | 1×/10 menit (429 + Retry-After) |
| Freeze | sesi `completed`/`in_progress` dan sesi < 24 jam tidak digeser otomatis |
| Pin | `ai_generated=false` tidak pernah disentuh regenerasi |
| Gagal | jadwal lama utuh, retry backoff (1m, 5m, 15m), tanpa error ke user |
| Idempotensi | replace per tanggal, bukan append |
| Snapshot | disimpan sebelum setiap regenerasi, retensi 7 hari |
| Horizon | MVP: 1 tanggal (hari ini) + rollover malam. Horizon 7 hari menunggu dukungan AI |

---

## 7. Rencana pengujian

**Unit**
- Debounce: 4 enqueue dalam 30 detik → 1 panggilan AI (fake client menghitung panggilan).
- Single-flight + dirty: enqueue saat berjalan → tepat 1 panggilan susulan.
- Rate limit manual: panggilan kedua < 10 menit → 429 + `Retry-After` + pesan.
- Filter locked: sesi pinned/frozen tidak terhapus & tidak tergeser.

**Integrasi (sqlite in-memory, pola test yang sudah ada)**
- Task create → sesi terbentuk (fake AI 200).
- AI mati → 0 sesi berubah, tidak ada error yang bocor ke response (jalur planner).
- AI 429 → jadwal lama utuh, tidak ada sesi parsial.
- Undo → jadwal kembali seperti snapshot.
- User edit sesi → `ai_generated=false` → regenerasi berikutnya tidak menyentuhnya.

**Regresi**
- `go test -race ./...`, `go vet ./...`, `go build ./...`, `gofmt -l .` bersih.
- Endpoint lama (generate manual, accept/reject, reschedule) tetap membalas seperti sekarang.

---

## 8. Fase implementasi

| Fase | Isi | Definition of done |
|---|---|---|
| **1. Fondasi** | migrasi kolom 4.1, Preload subject, `ai_source`/`ai_warnings` disimpan, `day_of_week` activity, predictor `class_level`/`curriculum`, TZ dari env | build/test hijau, sesi menyimpan metadata, AI menerima konteks lengkap |
| **2. Planner** | `workers/planner.go`, trigger hook, refactor context builder, persister transaksi | 4 edit = 1 panggilan; AI mati = jadwal utuh |
| **3. Kontrol user** | pin saat edit sesi, rate limit manual + pesan, snapshot + endpoint undo | sesi pinned aman; undo bekerja |
| **4. Otomatis harian** | rollover 00:05, retry backoff, logging | besok sudah terjadwal tanpa buka app |
| **5. Integrasi lanjutan** | forward feedback, horizon 7 hari (setelah AI siap) | feedback sampai ke AI |

---

## 9. Risiko & mitigasi

| Risiko | Mitigasi |
|---|---|
| Biaya LLM naik karena auto-trigger | debounce + single-flight + cap; hanya 1 tanggal di MVP |
| Jadwal "berubah sendiri" → user kehilangan kepercayaan | freeze 24 jam, pin, undo, toast informatif |
| Duplikasi sesi saat trigger bersamaan | single-flight per user + transaksi + replace per tanggal |
| AI lambat (LLM) membuat worker menumpuk | timeout `AI_TIMEOUT_SECONDS`, antrian per user, drop `dirty` bila user tidak aktif |
| Mobile lama error karena endpoint berubah | semua perubahan additive; accept/reject tetap ada |
| Server restart saat antrian berjalan | antrian in-memory (hilang) → rollover & trigger berikutnya memulihkan; tidak ada data korup karena transaksi |

---

## 10. Di luar lingkup (out of scope)

- Horizon 7 hari (butuh parameter baru di AI).
- Notifikasi push saat jadwal berubah (cukup toast di app).
- Multi-device realtime sync (tetap refetch).
- Perubahan logika penjadwalan di Go (tidak akan dilakukan).

---

## 11. File yang akan disentuh

```
migrations/migrations.go                 # kolom 4.1 + tabel snapshot
controllers/ai_controllers.go            # context builder, persister, metadata, manual limit, undo
controllers/task_controllers.go          # hook planner
controllers/subtask_controllers.go       # hook planner
controllers/schedule_controllers.go      # hook planner
controllers/activity_controllers.go      # hook planner + day_of_week
controllers/user_controllers.go          # hook planner (preferensi)
controllers/subject_controllers.go       # hook planner
controllers/session_controllers.go       # pin saat edit sesi
routes/routes.go                         # endpoint undo
workers/planner.go                       # BARU: antrian, debounce, single-flight
workers/scheduler.go                     # rollover 00:05 + pembersihan snapshot
main.go                                  # jalankan planner saat serve
docs/AI_CONTRACT.md, docs/API.md, docs/openapi.yaml, docs/DECISIONS.md (ADR baru)
```
