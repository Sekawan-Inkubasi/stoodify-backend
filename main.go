package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"stoodify_backend/config"
	"stoodify_backend/internal/seeder"
	"stoodify_backend/migrations"
	"stoodify_backend/routes"
	"stoodify_backend/workers"
)

// ==================== DEFAULT ====================

const (
	defaultUsers = 50
	defaultTasks = 40
	defaultPort  = "8080"
)

// ==================== ENTRY POINT ====================

func main() {
	// Tanpa argumen -> serve (paling sering dipakai waktu dev: `go run .`).
	if len(os.Args) < 2 {
		runServe(nil)
		return
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "serve", "start":
		runServe(args)
	case "migrate":
		runMigrate(args)
	case "migrate:fresh":
		runMigrateFresh(args)
	case "seed", "db:seed":
		runSeed(args)
	case "help", "-h", "--help", "-help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "command tidak dikenal: %q\n\n", cmd)
		printHelp()
		os.Exit(1)
	}
}

// ==================== COMMANDS ====================

// runServe: jalankan HTTP server + scheduler background.
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.String("port", envOr("PORT", defaultPort), "port HTTP server")
	fs.Usage = func() { printServeHelp(fs) }
	fs.Parse(args)

	db := config.KonekKeDB()

	// Scheduler: tandai tugas terlambat + notifikasi H-1/H-0.
	berhenti := make(chan struct{})
	go workers.JalankanScheduler(db, berhenti)

	// Graceful shutdown saat Ctrl+C / SIGTERM.
	sinyal := make(chan os.Signal, 1)
	signal.Notify(sinyal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sinyal
		log.Println("menerima sinyal berhenti")
		close(berhenti)
		os.Exit(0)
	}()

	router := routes.SetupRoutes()

	log.Printf("Server running on port %s", *port)
	if err := router.Run(":" + *port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}

// runMigrate: jalankan AutoMigrate + index kinerja. Opsional seed.
func runMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	withSeed := fs.Bool("seed", false, "jalankan seeder setelah migrate")
	users := fs.Int("users", defaultUsers, "jumlah user siswa (jika --seed)")
	tasks := fs.Int("tasks", defaultTasks, "jumlah tugas per user (jika --seed)")
	fs.Usage = func() { printMigrateHelp(fs) }
	fs.Parse(args)

	db := config.KonekKeDB()

	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("migrasi gagal: %v", err)
	}
	log.Println("migrasi selesai")

	if *withSeed {
		seeder.Run(db, seeder.Config{Users: *users, Tasks: *tasks})
	}
}

// runMigrateFresh: drop semua tabel, migrate ulang, opsional seed.
func runMigrateFresh(args []string) {
	fs := flag.NewFlagSet("migrate:fresh", flag.ExitOnError)
	withSeed := fs.Bool("seed", false, "jalankan seeder setelah migrate")
	users := fs.Int("users", defaultUsers, "jumlah user siswa (jika --seed)")
	tasks := fs.Int("tasks", defaultTasks, "jumlah tugas per user (jika --seed)")
	fs.Usage = func() { printMigrateFreshHelp(fs) }
	fs.Parse(args)

	db := config.KonekKeDB()

	if err := migrations.DropAll(db); err != nil {
		log.Fatalf("gagal drop tabel: %v", err)
	}
	log.Println("semua tabel dihapus")

	if err := migrations.Migrate(db); err != nil {
		log.Fatalf("migrasi gagal: %v", err)
	}
	log.Println("migrasi selesai")

	if *withSeed {
		seeder.Run(db, seeder.Config{Users: *users, Tasks: *tasks})
	}
}

// runSeed: isi database dengan data sintetis.
func runSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	users := fs.Int("users", defaultUsers, "jumlah user siswa")
	tasks := fs.Int("tasks", defaultTasks, "jumlah tugas per user")
	reset := fs.Bool("reset", false, "hapus data seed lama dulu")
	quiet := fs.Bool("quiet", false, "sembunyikan progres per tahap")
	fs.Usage = func() { printSeedHelp(fs) }
	fs.Parse(args)

	if *users < 1 || *tasks < 1 {
		log.Fatal("users dan tasks harus >= 1")
	}

	db := config.KonekKeDB()
	seeder.Run(db, seeder.Config{
		Users: *users,
		Tasks: *tasks,
		Reset: *reset,
		Quiet: *quiet,
	})
}

// ==================== HELP ====================

// printHelp: bantuan global untuk semua subcommand.
func printHelp() {
	fmt.Print(`Stoodify CLI

Usage:
  stoodify <command> [options]
  stoodify                          # default = serve

Commands:
  serve          Jalankan HTTP server + scheduler (default)
  migrate        Jalankan migrasi database (AutoMigrate + index kinerja)
  migrate:fresh  Drop semua tabel lalu migrasi ulang
  seed           Isi database dengan data sintetis untuk uji Mobile Dev
  help           Tampilkan bantuan ini

Global flags:
  -h, --help     Tampilkan bantuan untuk command tertentu

Contoh:
  stoodify serve
  stoodify serve --port=9090
  stoodify migrate
  stoodify migrate --seed --users=50 --tasks=40
  stoodify migrate:fresh --seed
  stoodify seed --users=200 --tasks=100 --reset
  stoodify seed --quiet
  stoodify migrate --help

Akun uji setelah seed:
  admin  : admin@stoodify.seed / rahasia123
  siswa  : user001@stoodify.seed s.d. userNNN@stoodify.seed / rahasia123
`)
}

func printServeHelp(fs *flag.FlagSet) {
	fmt.Print(`Usage: stoodify serve [options]

Jalankan HTTP server + scheduler background (penanda tugas terlambat,
notifikasi H-1/H-0). Port default dari env PORT atau 8080.

Options:
`)
	fs.PrintDefaults()
	fmt.Print(`
Contoh:
  stoodify serve
  stoodify serve --port=9090
`)
}

func printMigrateHelp(fs *flag.FlagSet) {
	fmt.Print(`Usage: stoodify migrate [options]

Menjalankan AutoMigrate semua model + membuat index kinerja komposit
(tasks user_id+status+deadline, schedules user_id+day_of_week, dst).

Options:
`)
	fs.PrintDefaults()
	fmt.Print(`
Contoh:
  stoodify migrate
  stoodify migrate --seed
  stoodify migrate --seed --users=200 --tasks=100
`)
}

func printMigrateFreshHelp(fs *flag.FlagSet) {
	fmt.Print(`Usage: stoodify migrate:fresh [options]

PERINGATAN: menghapus SEMUA tabel (termasuk data non-seed) lalu
migrasi ulang. Gunakan hanya di development.

Options:
`)
	fs.PrintDefaults()
	fmt.Print(`
Contoh:
  stoodify migrate:fresh
  stoodify migrate:fresh --seed
`)
}

func printSeedHelp(fs *flag.FlagSet) {
	fmt.Print(`Usage: stoodify seed [options]

Mengisi database dengan data sintetis. Semua user memakai domain email
@stoodify.seed sehingga tidak tercampur dengan akun asli dan bisa
dihapus bersih dengan --reset.

Options:
`)
	fs.PrintDefaults()
	fmt.Print(`
Contoh:
  stoodify seed
  stoodify seed --users=200 --tasks=100
  stoodify seed --users=50 --tasks=40 --reset
  stoodify seed --quiet
`)
}

// ==================== UTIL ====================

// envOr mengembalikan nilai env key, atau fallback bila kosong.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
