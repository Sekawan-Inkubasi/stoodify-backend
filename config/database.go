package config

import (
	"fmt"
	"log"
	"os"

	"stoodify_backend/migrations"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func KonekKeDB() *gorm.DB {
	if err := godotenv.Load(); err != nil {
		log.Println("INFO: .env file tidak ditemukan, pakai environment variables")
	}
	var dsn string
	dsn = NyariEnv("POSTGRES_URL", "")
	if dsn == "" {
		log.Fatalf("POSTGRES_URL masih kosong")
		username := NyariEnv("USER_DB", "")
		password := NyariEnv("PASS_DB", "")
		netloc := NyariEnv("NETLOCATION_DB", "")
		port := NyariEnv("PORT_DB", "")
		db_name := NyariEnv("DATABASE_NAME_DB", "")

		dsn = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", username, password, netloc, port, db_name)
	}

	var err error

	if DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{}); err != nil {
		log.Fatalf("gagal membuka konneksi database %v", err)
	}
	/*DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("gagal membuka koneksi database: %v", err)
	}*/

	if err := migrations.Migrate(DB); err != nil {
		log.Fatalf("gagal menjalankan migrasi: %v", err)
	}

	log.Println("SUKSES TERHUBUNG KE DATABASE")
	return DB

	// harusnya ini bagus kalau misal POSTGRES_URLnya gaada tapi gw males jir
	//
	//	if dsn == "" {
	//		log.Println("Databasenya gak nemu anjay")
	//	}
}

func NyariEnv(key string, replace_value string) string {

	_, oke := os.LookupEnv(key)
	if oke == true {
		return os.Getenv(key)
	} else {
		return replace_value
	}
}
