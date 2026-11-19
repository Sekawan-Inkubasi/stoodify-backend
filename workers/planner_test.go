package workers

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannerDebounceMenggabungkanPermintaan(t *testing.T) {
	var jumlah int32
	p := NewPlanner(80*time.Millisecond, func(userID uint) {
		atomic.AddInt32(&jumlah, 1)
	})
	defer p.Stop()

	// 5 permintaan beruntun (meniru user menambah 5 tugas cepat).
	for i := 0; i < 5; i++ {
		p.Enqueue(1)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&jumlah), "5 permintaan harus menjadi 1 regenerasi")
	assert.Equal(t, 1, p.JumlahRegenerasi(1))
}

func TestPlannerSingleFlightMenjalankanSekaliSusulan(t *testing.T) {
	var jumlah int32
	mulai := make(chan struct{})
	lepas := make(chan struct{})

	p := NewPlanner(10*time.Millisecond, func(userID uint) {
		atomic.AddInt32(&jumlah, 1)
		select {
		case <-mulai:
		default:
			close(mulai)
		}
		<-lepas // tahan regenerasi pertama agar permintaan berikutnya menumpuk sebagai dirty
	})
	defer p.Stop()

	p.Enqueue(2)
	<-mulai

	// Selama regenerasi berjalan, beberapa permintaan hanya menandai dirty.
	p.Enqueue(2)
	p.Enqueue(2)
	close(lepas)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&jumlah) == 2
	}, 2*time.Second, 20*time.Millisecond, "harus ada tepat satu regenerasi susulan")

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(2), atomic.LoadInt32(&jumlah), "tidak boleh ada regenerasi tambahan")
}

func TestPlannerUserBerbedaTidakSalingMenahan(t *testing.T) {
	var jumlah int32
	p := NewPlanner(10*time.Millisecond, func(userID uint) {
		atomic.AddInt32(&jumlah, 1)
	})
	defer p.Stop()

	p.Enqueue(10)
	p.Enqueue(11)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&jumlah) == 2
	}, 2*time.Second, 20*time.Millisecond)
}
