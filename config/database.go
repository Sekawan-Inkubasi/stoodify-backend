package config

import (
	"log"
	"os"
	"sync"

	"stoodify_backend/migrations"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// muatEnvSekali memastikan file .env hanya dibaca sekali per proses.
var muatEnvSekali sync.Once

// MuatEnvFile membaca .env lalu .env.local (menimpa nilai lama). Aman dipanggil
// berkali-kali: hanya eksekusi pertama yang benar-benar memuat file, supaya log
// "file tidak ditemukan" tidak dobel ketika `serve` memanggilnya sebelum KonekKeDB.
func MuatEnvFile() {
	muatEnvSekali.Do(func() {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("INFO: .env file tidak ditemukan, pakai environment variables")
		}
		_ = godotenv.Overload(".env.local")
	})
}

func KonekKeDB() *gorm.DB {
	MuatEnvFile()
	InitStorage()

	var dsn string
	dsn = NyariEnv("POSTGRES_URL", "")
	if dsn == "" {
		dsn = NyariEnv("DATABASE_URL", "")
	}
	if dsn == "" {
		dsn = NyariEnv("DATABASE_URL_UNPOOLED", "")
	}
	if dsn == "" {
		log.Fatalf("POSTGRES_URL / DATABASE_URL / DATABASE_URL_UNPOOLED masih kosong")
	}

	var err error

	if DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{}); err != nil {
		log.Fatalf("gagal membuka konneksi database %v", err)
	}

	if err := migrations.Migrate(DB); err != nil {
		log.Fatalf("gagal menjalankan migrasi: %v", err)
	}

	log.Println("SUKSES TERHUBUNG KE DATABASE")
	return DB
}

func NyariEnv(key string, replace_value string) string {
	_, oke := os.LookupEnv(key)
	if oke == true {
		return os.Getenv(key)
	} else {
		return replace_value
	}
}
