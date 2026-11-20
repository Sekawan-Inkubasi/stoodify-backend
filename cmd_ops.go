package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"stoodify_backend/config"
	"stoodify_backend/migrations"
)

// ==================== BUILD INFO ====================
// Diisi lewat -ldflags saat build/push image:
//
//	-X main.version=1.2.0 -X main.commit=abc1234 -X main.buildDate=2026-11-20T10:00:00Z
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// runVersion mencetak informasi build — dipakai untuk memastikan versi apa yang
// benar-benar berjalan di server (pertanyaan pertama saat insiden).
func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "cetak sebagai JSON")
	fs.Parse(args)

	info := map[string]string{
		"version":    version,
		"commit":     commit,
		"build_date": buildDate,
		"go":         runtime.Version(),
		"os_arch":    runtime.GOOS + "/" + runtime.GOARCH,
		"app_env":    envOr("APP_ENV", "development"),
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
		return
	}
	fmt.Printf("stoodify %s\n", version)
	fmt.Printf("  commit     : %s\n", commit)
	fmt.Printf("  build date : %s\n", buildDate)
	fmt.Printf("  go         : %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  app env    : %s\n", envOr("APP_ENV", "development"))
}

// ==================== DOCTOR / PREFLIGHT ====================

type cek struct {
	Nama   string `json:"name"`
	Status string `json:"status"` // ok | warning | error
	Pesan  string `json:"message"`
	Kritis bool   `json:"critical"`
}

// runDoctor menjalankan pemeriksaan kesiapan lingkungan (env, DB, AI, SMTP).
// Dipakai sebelum deploy atau saat service bermasalah: `stoodify doctor`.
func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "cetak hasil sebagai JSON")
	skipDB := fs.Bool("skip-db", false, "lewati pemeriksaan database")
	timeout := fs.Duration("timeout", 3*time.Second, "batas waktu cek konektivitas per target")
	fs.Parse(args)

	config.MuatEnvFile()

	var hasil []cek
	tambah := func(nama, status, pesan string, kritis bool) {
		hasil = append(hasil, cek{Nama: nama, Status: status, Pesan: pesan, Kritis: kritis})
	}

	// --- env wajib ---
	dsn := NyariDSN()
	if dsn == "" {
		tambah("env.POSTGRES_URL", "error", "POSTGRES_URL / DATABASE_URL / DATABASE_URL_UNPOOLED kosong", true)
	} else {
		tambah("env.POSTGRES_URL", "ok", "terisi (nilai disembunyikan)", false)
	}
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		tambah("env.JWT_SECRET", "error", "kosong: semua token akan gagal dibuat/diverifikasi", true)
	} else {
		tambah("env.JWT_SECRET", "ok", "terisi", false)
	}
	for _, key := range []string{"AI_SCHEDULER_URL", "AI_PREDICTOR_URL"} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			tambah("env."+key, "error", "kosong: perintah `serve` akan menolak start (bridge-only)", true)
		} else {
			tambah("env."+key, "ok", os.Getenv(key), false)
		}
	}

	// --- peringatan (tidak menggagalkan start, tapi fitur mati diam-diam) ---
	if strings.TrimSpace(os.Getenv("SMTP_HOST")) == "" {
		tambah("env.SMTP_HOST", "warning",
			"kosong: NoopMailer aktif — email verifikasi & reset password TIDAK terkirim, pendaftar baru tidak bisa login", false)
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")) == "" {
		tambah("env.GOOGLE_CLIENT_ID", "warning", "kosong: POST /auth/google membalas 503", false)
	}
	if envOr("APP_ENV", "development") != "production" {
		tambah("env.APP_ENV", "warning",
			"bukan production: detail error internal ikut dikirim ke client (development)", false)
	}
	if strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")) == "" && envOr("APP_ENV", "development") == "production" {
		tambah("env.TRUSTED_PROXIES", "warning",
			"kosong di production: X-Forwarded-For tidak dipercaya — semua IP terlihat sama bagi rate limiter", false)
	}

	// --- konektivitas AI (peringatan saja: service AI boleh sedang mati) ---
	for _, key := range []string{"AI_SCHEDULER_URL", "AI_PREDICTOR_URL"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		if err := cekTCP(raw, *timeout); err != nil {
			tambah("konektivitas."+key, "warning", "tidak dapat dihubungi: "+err.Error(), false)
		} else {
			tambah("konektivitas."+key, "ok", "host terjangkau", false)
		}
	}

	// --- database ---
	if !*skipDB && dsn != "" {
		if err := cekDatabase(dsn); err != nil {
			tambah("database", "error", err.Error(), true)
		} else {
			tambah("database", "ok", "terhubung, tabel inti tersedia", false)
		}
	}

	// --- output ---
	adaError := false
	for _, h := range hasil {
		if h.Status == "error" {
			adaError = true
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"ok": !adaError, "checks": hasil})
	} else {
		for _, h := range hasil {
			ikon := map[string]string{"ok": "[ OK ]", "warning": "[WARN]", "error": "[FAIL]"}[h.Status]
			fmt.Printf("%s %-28s %s\n", ikon, h.Nama, h.Pesan)
		}
		fmt.Println()
		if adaError {
			fmt.Println("hasil: GAGAL — perbaiki item [FAIL] sebelum menjalankan serve.")
		} else {
			fmt.Println("hasil: siap (peringatan tidak menghalangi start, tapi mematikan fitur terkait).")
		}
	}

	if adaError {
		os.Exit(1)
	}
}

// cekTCP memastikan host:port dari URL dapat dijangkau.
func cekTCP(rawURL string, timeout time.Duration) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL tidak valid: %w", err)
	}
	host := u.Host
	if host == "" {
		host = u.Path
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		if u.Scheme == "https" {
			host = net.JoinHostPort(host, "443")
		} else {
			host = net.JoinHostPort(host, "80")
		}
	}
	koneksi, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return err
	}
	return koneksi.Close()
}

// cekDatabase memastikan DSN bisa dipakai dan tabel inti sudah ada.
func cekDatabase(dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("DSN tidak valid: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping gagal: %w", err)
	}
	var jumlah int
	err = db.QueryRow(`SELECT count(*) FROM information_schema.tables
	                   WHERE table_schema = 'public' AND table_name IN ('users','tasks','study_sessions')`).Scan(&jumlah)
	if err != nil {
		return fmt.Errorf("gagal membaca skema: %w", err)
	}
	if jumlah < 3 {
		return fmt.Errorf("tabel inti belum lengkap (%d/3) — jalankan `stoodify migrate`", jumlah)
	}
	return nil
}

// NyariDSN mengembalikan DSN dari env dengan urutan yang sama seperti KonekKeDB.
func NyariDSN() string {
	for _, key := range []string{"POSTGRES_URL", "DATABASE_URL", "DATABASE_URL_UNPOOLED"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// ==================== BACKUP / RESTORE / ROLLBACK ====================
// Catatan: ini rollback DATA (database). Rollback aplikasi tetap lewat git/CI
// (`git reset --hard <sha>` lalu rebuild image), karena binary tidak bisa
// menurunkan versi dirinya sendiri dengan aman.

func runBackup(args []string) {
	fs := flag.NewFlagSet("db:backup", flag.ExitOnError)
	dir := fs.String("dir", "backups", "direktori tujuan backup")
	keep := fs.Int("keep", 7, "jumlah backup terbaru yang disimpan (0 = simpan semua)")
	quiet := fs.Bool("quiet", false, "hanya cetak path hasil")
	fs.Parse(args)

	dsn := muatDSNUntukOperasi()
	if _, err := exec.LookPath("pg_dump"); err != nil {
		log.Fatal("pg_dump tidak ditemukan di PATH. Pasang paket postgresql-client di server/container.")
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatalf("gagal membuat direktori backup: %v", err)
	}

	nama := "stoodify-" + time.Now().Format("20060102-150405") + ".sql.gz"
	tujuan := filepath.Join(*dir, nama)

	berkas, err := os.Create(tujuan)
	if err != nil {
		log.Fatalf("gagal membuat file backup: %v", err)
	}
	gz := gzip.NewWriter(berkas)

	// --clean --if-exists: file backup membawa DROP sebelum CREATE, sehingga
	// restore ke database yang masih berisi tabel menjadi idempotent (tanpa ini
	// restore gagal dengan "relation already exists").
	cmd := exec.Command("pg_dump", "--no-owner", "--no-privileges", "--clean", "--if-exists", "--dbname="+dsn)
	cmd.Stdout = gz
	var stderr strings.Builder
	cmd.Stderr = &stderr

	jalankanErr := cmd.Run()
	_ = gz.Close()
	_ = berkas.Close()

	if jalankanErr != nil {
		_ = os.Remove(tujuan)
		log.Fatalf("pg_dump gagal: %v\n%s", jalankanErr, strings.TrimSpace(stderr.String()))
	}

	info, _ := os.Stat(tujuan)
	if !*quiet {
		fmt.Printf("backup selesai: %s (%.1f MB)\n", tujuan, float64(info.Size())/(1024*1024))
	} else {
		fmt.Println(tujuan)
	}

	if *keep > 0 {
		hapusBackupLama(*dir, *keep, *quiet)
	}
}

func runRestore(args []string) {
	fs := flag.NewFlagSet("db:restore", flag.ExitOnError)
	berkas := fs.String("file", "", "file backup .sql.gz yang akan dipulihkan")
	terbaru := fs.Bool("latest", false, "pakai backup terbaru di --dir")
	dir := fs.String("dir", "backups", "direktori backup (dipakai dengan --latest)")
	paksa := fs.Bool("force", false, "wajib di production: menimpa data yang ada")
	fs.Parse(args)

	dsn := muatDSNUntukOperasi()

	target := strings.TrimSpace(*berkas)
	if target == "" && *terbaru {
		daftar, err := daftarBackup(*dir)
		if err != nil || len(daftar) == 0 {
			log.Fatalf("tidak ada backup di %s", *dir)
		}
		target = filepath.Join(*dir, daftar[0].Nama)
	}
	if target == "" {
		log.Fatal("tentukan --file=<path> atau --latest")
	}
	if _, err := os.Stat(target); err != nil {
		log.Fatalf("file backup tidak ditemukan: %s", target)
	}

	if envOr("APP_ENV", "development") == "production" && !*paksa {
		log.Fatalf("menolak restore di production tanpa --force (data saat ini akan ditimpa)")
	}
	if _, err := exec.LookPath("psql"); err != nil {
		log.Fatal("psql tidak ditemukan di PATH. Pasang paket postgresql-client.")
	}

	fmt.Printf("memulihkan %s ...\n", target)

	sumber, err := os.Open(target)
	if err != nil {
		log.Fatalf("gagal membuka backup: %v", err)
	}
	defer sumber.Close()

	gz, err := gzip.NewReader(sumber)
	if err != nil {
		log.Fatalf("file bukan gzip valid: %v", err)
	}
	defer gz.Close()

	cmd := exec.Command("psql", "--dbname="+dsn, "--set=ON_ERROR_STOP=1", "--quiet")
	cmd.Stdin = gz
	cmd.Stdout = io.Discard
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("restore gagal: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}

	fmt.Println("restore selesai. Jalankan `stoodify migrate` untuk menyelaraskan skema terbaru.")
}

// runRollback mengembalikan database ke backup terakhir (atau nomor tertentu).
func runRollback(args []string) {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	dir := fs.String("dir", "backups", "direktori backup")
	langkah := fs.Int("step", 1, "1 = backup terbaru, 2 = sebelum itu, dst.")
	daftar := fs.Bool("list", false, "tampilkan daftar backup lalu keluar")
	paksa := fs.Bool("force", false, "wajib di production")
	fs.Parse(args)

	backup, err := daftarBackup(*dir)
	if err != nil {
		log.Fatalf("gagal membaca direktori backup: %v", err)
	}
	if len(backup) == 0 {
		log.Fatalf("tidak ada backup di %s — buat dulu dengan `stoodify db:backup`", *dir)
	}

	if *daftar {
		fmt.Printf("%-32s %10s  %s\n", "NAMA", "UKURAN", "DIBUAT")
		for _, b := range backup {
			fmt.Printf("%-32s %9.1fM  %s\n", b.Nama, float64(b.Ukuran)/(1024*1024),
				b.Waktu.Format("2006-01-02 15:04:05"))
		}
		return
	}

	if *langkah < 1 || *langkah > len(backup) {
		log.Fatalf("--step=%d di luar jangkauan (tersedia 1..%d)", *langkah, len(backup))
	}
	target := filepath.Join(*dir, backup[*langkah-1].Nama)

	fmt.Printf("rollback database ke: %s (%s)\n", target, backup[*langkah-1].Waktu.Format("2006-01-02 15:04:05"))
	fmt.Println("catatan: rollback KODE/aplikasi dilakukan lewat CI (git reset --hard <sha> lalu deploy ulang).")

	argumen := []string{"--file=" + target}
	if *paksa {
		argumen = append(argumen, "--force")
	}
	runRestore(argumen)

	// Pastikan skema tetap mutakhir setelah restore dari backup lama.
	config.MuatEnvFile()
	db := config.KonekKeDB()
	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("migrasi setelah rollback gagal: %v", err)
	}
	fmt.Println("skema diselaraskan ke versi terbaru.")
}

type infoBackup struct {
	Nama   string
	Ukuran int64
	Waktu  time.Time
}

// daftarBackup mengembalikan backup di direktori, terbaru lebih dulu.
func daftarBackup(dir string) ([]infoBackup, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var hasil []infoBackup
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "stoodify-") || !strings.HasSuffix(e.Name(), ".sql.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		hasil = append(hasil, infoBackup{Nama: e.Name(), Ukuran: info.Size(), Waktu: info.ModTime()})
	}
	sort.Slice(hasil, func(i, j int) bool { return hasil[i].Waktu.After(hasil[j].Waktu) })
	return hasil, nil
}

// hapusBackupLama menyisakan `keep` backup terbaru.
func hapusBackupLama(dir string, keep int, quiet bool) {
	daftar, err := daftarBackup(dir)
	if err != nil || len(daftar) <= keep {
		return
	}
	for _, b := range daftar[keep:] {
		if err := os.Remove(filepath.Join(dir, b.Nama)); err == nil && !quiet {
			fmt.Printf("backup lama dihapus: %s\n", b.Nama)
		}
	}
}

// muatDSNUntukOperasi memuat .env lalu memastikan DSN tersedia.
func muatDSNUntukOperasi() string {
	config.MuatEnvFile()
	dsn := NyariDSN()
	if dsn == "" {
		log.Fatal("POSTGRES_URL / DATABASE_URL / DATABASE_URL_UNPOOLED kosong")
	}
	return dsn
}
