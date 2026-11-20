package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNyariDSNUrutanPrioritas(t *testing.T) {
	t.Setenv("POSTGRES_URL", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_URL_UNPOOLED", "")
	assert.Empty(t, NyariDSN())

	t.Setenv("DATABASE_URL_UNPOOLED", "postgres://unpooled")
	assert.Equal(t, "postgres://unpooled", NyariDSN())

	t.Setenv("DATABASE_URL", "postgres://database")
	assert.Equal(t, "postgres://database", NyariDSN())

	t.Setenv("POSTGRES_URL", "postgres://postgres")
	assert.Equal(t, "postgres://postgres", NyariDSN(), "POSTGRES_URL harus menang")
}

func TestDaftarBackupUrutTerbaruDanFilter(t *testing.T) {
	dir := t.TempDir()

	buat := func(nama string, umur time.Duration) {
		p := filepath.Join(dir, nama)
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
		require.NoError(t, os.Chtimes(p, time.Now().Add(-umur), time.Now().Add(-umur)))
	}

	buat("stoodify-20260101-000000.sql.gz", 3*time.Hour)
	buat("stoodify-20260102-000000.sql.gz", 1*time.Hour)
	buat("stoodify-20260103-000000.sql.gz", 2*time.Hour)
	buat("catatan.txt", time.Minute)                    // bukan backup
	buat("lainnya-20260101-000000.sql.gz", time.Minute) // prefix salah
	buat("stoodify-20260104-000000.sql", time.Minute)   // bukan .gz

	daftar, err := daftarBackup(dir)
	require.NoError(t, err)
	require.Len(t, daftar, 3, "hanya file stoodify-*.sql.gz yang dihitung")

	assert.Equal(t, "stoodify-20260102-000000.sql.gz", daftar[0].Nama, "terbaru lebih dulu")
	assert.Equal(t, "stoodify-20260103-000000.sql.gz", daftar[1].Nama)
	assert.Equal(t, "stoodify-20260101-000000.sql.gz", daftar[2].Nama)
}

func TestDaftarBackupDirektoriTidakAda(t *testing.T) {
	daftar, err := daftarBackup(filepath.Join(t.TempDir(), "belum-ada"))
	assert.NoError(t, err, "direktori belum ada bukan error")
	assert.Empty(t, daftar)
}

func TestHapusBackupLamaMenyisakanN(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		nama := filepath.Join(dir, "stoodify-2026010"+string(rune('1'+i))+"-000000.sql.gz")
		require.NoError(t, os.WriteFile(nama, []byte("x"), 0o644))
		waktu := time.Now().Add(-time.Duration(5-i) * time.Hour)
		require.NoError(t, os.Chtimes(nama, waktu, waktu))
	}

	hapusBackupLama(dir, 2, true)

	daftar, err := daftarBackup(dir)
	require.NoError(t, err)
	assert.Len(t, daftar, 2, "hanya 2 backup terbaru yang disisakan")
	// Umur: 20260105 paling baru (1 jam), 20260101 paling lama (5 jam).
	assert.Equal(t, "stoodify-20260105-000000.sql.gz", daftar[0].Nama)
	assert.Equal(t, "stoodify-20260104-000000.sql.gz", daftar[1].Nama)
}

func TestHapusBackupLamaTidakMenghapusSaatCukup(t *testing.T) {
	dir := t.TempDir()
	nama := filepath.Join(dir, "stoodify-20260101-000000.sql.gz")
	require.NoError(t, os.WriteFile(nama, []byte("x"), 0o644))

	hapusBackupLama(dir, 7, true)

	_, err := os.Stat(nama)
	assert.NoError(t, err, "backup tidak boleh dihapus kalau jumlahnya belum melebihi batas")
}

func TestCekTCPMenolakURLTakTerjangkau(t *testing.T) {
	err := cekTCP("http://127.0.0.1:1/health", 300*time.Millisecond)
	assert.Error(t, err, "port 1 tidak mungkin menerima koneksi")
}

func TestCekTCPURLTidakValid(t *testing.T) {
	assert.Error(t, cekTCP("://bukan-url", time.Second))
}
