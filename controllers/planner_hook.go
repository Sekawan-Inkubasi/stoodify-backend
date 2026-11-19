package controllers

// Pemicu regenerasi jadwal otomatis.
//
// Controller tidak tahu detail planner; main.go menanamkan implementasinya
// (workers.Planner) lewat SetPemicuRegenerasi. Kalau belum di-set (mis. saat test
// unit), pemanggilan menjadi no-op sehingga controller tetap bisa diuji tanpa AI.

// PemicuRegenerasi adalah antarmuka minimal yang dibutuhkan controller.
type PemicuRegenerasi interface {
	Enqueue(userID uint)
}

var pemicuRegenerasi PemicuRegenerasi

// SetPemicuRegenerasi dipanggil sekali dari main.go saat server start.
func SetPemicuRegenerasi(p PemicuRegenerasi) {
	pemicuRegenerasi = p
}

// picuRegenerasi meminta planner menyusun ulang jadwal user (fire-and-forget).
func picuRegenerasi(userID uint) {
	if pemicuRegenerasi == nil || userID == 0 {
		return
	}
	pemicuRegenerasi.Enqueue(userID)
}
