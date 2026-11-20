# TODO Divisi Mobile

Scope: aplikasi mobile (`stoodify-mobile` / `stoodify-apk`). Backend tidak mengerjakan item di sini.
Rujukan kontrak backend: [`docs/API.md`](../API.md), [`docs/PLAN-AUTO-SCHEDULE.md`](../PLAN-AUTO-SCHEDULE.md), dan [`docs/DECISIONS.md`](../DECISIONS.md) (ADR-008 s.d. ADR-010).

**Dua kelompok pekerjaan:**

- **Bagian A (poin 1–9)** — penjadwalan otomatis: jadwal AI sekarang **diterapkan otomatis** oleh backend setiap ada perubahan tugas. Tidak ada lagi alur "AI mengusulkan → user menerima".
- **Bagian B (poin 10–13)** — akun: verifikasi email wajib, login dengan Google, onboarding profil, dan field register yang wajib.

Kerjakan **Bagian B lebih dulu**: tanpanya user baru tidak bisa login sama sekali.

### Ringkasan endpoint akun (Bagian B)

| Endpoint | Dipakai untuk |
| --- | --- |
| `POST /api/auth/register` | Daftar (wajib `school`/`grade`/`major`) → **tidak** memberi token |
| `POST /api/auth/verify-email` | Kirim `{token}` dari deep link |
| `POST /api/auth/verify-email/resend` | Kirim ulang link (balasan selalu generik) |
| `POST /api/auth/google` | Kirim `{id_token}` dari Google Sign-In SDK |
| `POST /api/auth/login` | `{email,password}` **atau** `{id_token}` |
| `PATCH /api/users/me/onboarding` | Lengkapi `school`/`grade`/`major` (akun Google) |

Halaman `GET /verify-email?token=...` (di luar `/api`) hanya jembatan ke deep link — aplikasi **tidak** perlu membukanya.

---

## 1. Hapus alur terima/tolak

- [ ] Hapus tombol **Terima** dan **Tolak** per sesi di `AIPlanView.tsx`.
- [ ] Hapus state/badge "menunggu persetujuan"; semua sesi yang tampil sudah final.
- [ ] Hapus pemanggilan `POST /ai/schedule-recommendations/{id}/accept` dan `/reject` dari client.
- [ ] Ganti label layar dari "Rekomendasi AI" menjadi **"Jadwal Belajar"**.
- [ ] Hapus loading/empty state khusus "AI sedang menyusun rekomendasi"; ganti dengan skeleton biasa.

**Acceptance:** tidak ada teks/tombol yang menyiratkan user harus menyetujui jadwal.

## 2. Toast + Urungkan

- [ ] Saat aplikasi mendeteksi jadwal berubah (refetch setelah simpan tugas), tampilkan toast non-blocking: **"Jadwal diperbarui karena ada tugas baru"**.
- [ ] Toast punya dua aksi: **[Lihat]** (scroll ke sesi hari ini) dan **[Urungkan]** (panggil `POST /api/ai/schedule-recommendations/undo`).
- [ ] Auto-dismiss ±10 detik; kalau user menekan Urungkan setelah kedaluwarsa, tampilkan pesan "Waktu membatalkan sudah lewat".
- [ ] Jangan pakai modal/dialog konfirmasi untuk perubahan jadwal otomatis.
- [ ] Kalau `undo` gagal (404 karena snapshot habis), tampilkan: "Jadwal sebelumnya sudah tidak tersedia".

**Acceptance:** perubahan jadwal otomatis tidak pernah memblokir interaksi user.

## 3. Penanda sesi

- [ ] Beri label halus **"baru"** pada sesi yang baru dibuat/digeser (bandingkan `updated_at`/`plan_version`).
- [ ] Tampilkan ikon kecil **"diubah"** pada sesi hasil editan user (pinned).
- [ ] Kalau `ai_source = "fallback"`, tampilkan keterangan netral: **"Jadwal disusun dari rencana cadangan"** (jangan sebut "AI gagal").

**Acceptance:** user bisa membedakan sesi otomatis vs sesi yang dia ubah sendiri tanpa membaca dokumentasi.

## 4. Edit sesi = pin

- [ ] Saat user mengubah jam/tanggal sesi (`PATCH /api/study-sessions/:id`), backend otomatis menandainya pinned — client hanya perlu mengirim perubahan seperti biasa.
- [ ] Setelah edit, tampilkan toast: **"Sesi ini kamu atur sendiri — tidak akan diubah otomatis."**
- [ ] Jelaskan sekali (info tooltip/first-time hint) bahwa sesi yang diubah tidak ikut disusun ulang.

**Acceptance:** mengedit sesi tidak memicu peringatan/konflik apa pun, dan sesi itu tetap di tempatnya setelah regenerasi berikutnya.

## 5. Tombol "Susun ulang" manual

- [ ] Tombol tetap ada, tapi hanya untuk kasus user ingin menyusun ulang seluruh hari.
- [ ] Tangani `429` + `Retry-After`: tampilkan pesan dari backend apa adanya (`message`), plus hitung mundur.
- [ ] Tangani `200` "tidak ada yang berubah": tampilkan **"Jadwal kamu sudah paling pas"**.
- [ ] Tangani `502`/`503`: **"AI sedang sibuk. Jadwal lama tetap dipakai."** tanpa menghapus tampilan jadwal.
- [ ] Nonaktifkan tombol sementara selama request berjalan (hindari double tap).

**Acceptance:** spam tombol tidak menghasilkan lebih dari satu request efektif dan pesannya jelas.

## 6. Sinkronisasi data

- [ ] Refetch `GET /api/study-sessions/?status=planned` setelah setiap simpan/ubah/hapus tugas, ubah jadwal sekolah, aktivitas, dan preferensi.
- [ ] Refetch saat app kembali dari background (resume) dan saat pull-to-refresh.
- [ ] Tidak perlu websocket/push untuk MVP.

**Acceptance:** jadwal yang tampil selalu ≤ 1 aksi user dari kondisi server.

## 7. Data baru yang perlu ditampilkan/dipakai

- [ ] `ai_source` (opsional): untuk label cadangan di poin 3.
- [ ] `ai_warnings` (opsional): sembunyikan dari user, cukup untuk debugging internal.
- [ ] `day_of_week` pada form aktivitas (dropdown Senin–Minggu) — tanpa ini kegiatan mingguan tidak akurat.

**Acceptance:** form aktivitas mengirim `day_of_week` saat recurrence `weekly`.

## 8. Empty & error state

- [ ] Belum ada tugas → "Belum ada tugas. Tambah tugas pertama kamu, jadwal belajar akan dibuat otomatis."
- [ ] Tugas ada tapi jadwal kosong (AI belum selesai) → skeleton + teks "Sedang menyusun jadwal…" (maks 5 detik), lalu fallback ke teks netral tanpa menyalahkan AI.

## 9. Pengujian

- [ ] Simpan 4 tugas beruntun → UI hanya menampilkan satu toast pembaruan.
- [ ] Matikan service AI → simpan tugas → jadwal lama tetap tampil, tidak ada error dialog.
- [ ] Tekan "Susun ulang" dua kali cepat → hanya satu request yang efektif, pesan limit tampil benar.
- [ ] Edit sesi → regenerasi (simpan tugas lain) → sesi hasil edit tidak berpindah.
- [ ] Tekan "Urungkan" → jadwal kembali ke kondisi sebelum perubahan.

---

## 10. Verifikasi Email (wajib sebelum login)

Backend: `POST /auth/register` sekarang **tidak** memberi token. Akun berstatus `email_verified=false` sampai user mengeklik link di email.

- [ ] Setelah register **201**, jangan simpan token (tidak ada), langsung tampilkan layar **"Cek email kamu"** berisi email yang dipakai.
- [ ] Tombol **Kirim ulang** → `POST /api/auth/verify-email/resend { email }`. Balasannya selalu 200 generik; tampilkan "Link baru sudah dikirim" tanpa mengklaim email pasti ada.
- [ ] Tangani login **403** + `errors.email_verified = "false"` → arahkan ke layar "Cek email kamu", **bukan** pesan "email atau password salah".
- [ ] Tangani login **401** "Akun ini terdaftar lewat Google" → tampilkan tombol **Masuk dengan Google**.
- [ ] Batasi tombol kirim ulang (mis. 60 detik) supaya tidak spam.
- [ ] Token kedaluwarsa/sudah dipakai (400 dari `/verify-email`) → tampilkan pesan + tombol kirim ulang.

### Deep link `stoodify://verify-email`

- [ ] Daftarkan skema di `AndroidManifest.xml` (intent-filter `BROWSABLE`, scheme `stoodify`, host `verify-email`).
- [ ] Daftarkan URL Type `stoodify` di `Info.plist` (iOS).
- [ ] Bagian **Universal/App Links** (opsional, lebih mulus): host `api.ezzar.my.id`, path `/verify-email`; kalau dipakai, backend perlu file `assetlinks.json` / `apple-app-site-association` — koordinasi dengan backend dulu.
- [ ] Handler deep link: ambil `token` → `POST /api/auth/verify-email { token }` → sukses: "Email terverifikasi" + tombol **Masuk**.
- [ ] Kalau dibuka saat app sudah login (kasus langka), cukup tampilkan hasil verifikasi tanpa mengubah sesi.

**Acceptance:** user yang mendaftar dari HP bisa menyelesaikan verifikasi tanpa membuka browser manual dan tanpa mengetik apa pun.

## 11. Login / Daftar dengan Google

Backend: `POST /auth/google { id_token }` — menerima **ID token** dari Google Sign-In SDK (bukan access token).

- [ ] Pasang Google Sign-In SDK (Android: Credential Manager / `google_sign_in`; iOS: Google Sign-In). Pakai **client ID platform masing-masing**, bukan client ID web.
- [ ] Kirim `id_token` ke `POST /api/auth/google`, simpan `access_token` + `refresh_token`.
- [ ] `needs_onboarding == true` → tampilkan layar **Sekolah / Kelas / Jurusan** (poin 12). `false` → langsung beranda.
- [ ] Tangani `409` "email ini sudah terhubung ke akun Google lain" → sarankan login dengan cara sebelumnya.
- [ ] Tangani `503` "login Google belum dikonfigurasi di server" → sembunyikan tombol Google atau tampilkan pesan netral (jangan crash).
- [ ] `POST /auth/login` juga menerima `{ id_token }`; pilih satu jalur saja agar tidak duplikat logika.

**Acceptance:** akun baru lewat Google langsung dapat sesi, dan layar onboarding hanya muncul sekali sampai profil lengkap.

## 12. Onboarding (sekolah / kelas / jurusan)

- [ ] Form 3 field: `school` (teks), `grade` (10-13), `major` (teks). Validasi klien **dan** tampilkan `errors` per field dari server.
- [ ] Submit → `PATCH /api/users/me/onboarding { school, grade, major }` (pakai access token yang sudah ada).
- [ ] Tangani **409** dari endpoint lain dengan `errors.profile_completed = "false"` → lempar balik ke layar ini (jangan tampilkan sebagai error umum).
- [ ] Setelah sukses, refresh `GET /api/users/me` lalu masuk beranda.
- [ ] Jangan izinkan "lewati": fitur AI & statistik memang terkunci sampai profil lengkap.

**Acceptance:** tidak ada jalan bagi user untuk sampai ke fitur AI dengan `profile_completed=false`.

## 13. Register: field wajib

- [ ] Form daftar **wajib** punya `school`, `grade`, `major` — backend menolak tanpa itu (`errors.school/grade/major: "wajib diisi"`).
- [ ] Tampilkan pesan error per field dari `errors`, jangan pesan generik "gagal mendaftar".

**Acceptance:** tidak ada request register yang berakhir 400 hanya karena form kurang field.

---

## Catatan koordinasi (revisi)

- Endpoint `accept`/`reject` **masih ada** di backend untuk sementara (deprecated). Jangan dipakai lagi, tapi jangan panik kalau masih muncul di collection Bruno lama.
- Kontrak payload/response tidak berubah selain tambahan field opsional (`ai_source`, `ai_warnings`, `plan_version`).
- Perubahan urutan kerja: backend fase 1–3 dulu, mobile bisa mulai poin 1, 2, 5, 6 paralel karena tidak menunggu endpoint baru (kecuali `undo`).
- **PATCH kini parsial:** `PATCH /users/me`, `/subjects/:id`, `/schedules/:id`, `/tasks/:id`, `/activities/:id` hanya mengubah field yang dikirim. Aman mengirim satu field saja.
- **Aktivitas:** selalu kirim `day_of_week` (Senin–Minggu) saat `recurrence = weekly`, kalau tidak AI menganggap kegiatan berlangsung setiap hari.
- **Error validasi:** `errors` berisi pesan Indonesia per field (`{"school": "wajib diisi"}`). Di production tidak ada lagi teks mentah seperti `RegisterInput.School`.
