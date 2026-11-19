package workers

import (
	"sync"
	"time"
)

// Planner: antrian regenerasi jadwal per user.
//
// Tujuan: perubahan tugas yang beruntun (mis. user menambah 4 tugas dalam 30 detik)
// tidak menghasilkan 4 panggilan AI. Aturannya:
//   - debounce: permintaan ditunda sebentar agar permintaan berikutnya menimpa yang lama
//   - single-flight: hanya satu regenerasi berjalan per user
//   - dirty: permintaan yang datang saat regenerasi berjalan dijalankan SEKALI setelahnya
//
// Planner tidak tahu apa pun soal AI atau database; pekerjaan sebenarnya diberikan
// lewat callback `run` saat konstruksi (di-wire dari main.go).
type Planner struct {
	debounce time.Duration
	run      func(userID uint)

	mu     sync.Mutex
	states map[uint]*plannerState
	stop   chan struct{}
	closed bool
}

type plannerState struct {
	timer   *time.Timer
	running bool
	dirty   bool
	runs    int
}

// NewPlanner membuat planner dengan debounce tertentu dan fungsi kerja per user.
func NewPlanner(debounce time.Duration, run func(userID uint)) *Planner {
	if debounce < 0 {
		debounce = 0
	}
	return &Planner{
		debounce: debounce,
		run:      run,
		states:   make(map[uint]*plannerState),
		stop:     make(chan struct{}),
	}
}

// Enqueue meminta regenerasi untuk user. Aman dipanggil berkali-kali.
func (p *Planner) Enqueue(userID uint) {
	if userID == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}

	st := p.states[userID]
	if st == nil {
		st = &plannerState{}
		p.states[userID] = st
	}

	// Sudah ada yang jalan: cukup tandai, nanti dijalankan sekali lagi.
	if st.running {
		st.dirty = true
		return
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	st.timer = time.AfterFunc(p.debounce, func() { p.jalankan(userID) })
}

// Stop menghentikan planner (timer yang menunggu dibatalkan).
func (p *Planner) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	for _, st := range p.states {
		if st.timer != nil {
			st.timer.Stop()
		}
	}
	close(p.stop)
}

// JumlahRegenerasi: berapa kali user ini benar-benar memicu panggilan AI (untuk test/observability).
func (p *Planner) JumlahRegenerasi(userID uint) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if st := p.states[userID]; st != nil {
		return st.runs
	}
	return 0
}

func (p *Planner) jalankan(userID uint) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	st := p.states[userID]
	if st == nil {
		p.mu.Unlock()
		return
	}
	if st.running {
		st.dirty = true
		p.mu.Unlock()
		return
	}
	st.running = true
	st.dirty = false
	st.runs++
	p.mu.Unlock()

	if p.run != nil {
		p.run(userID)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	st.running = false
	if st.dirty && !p.closed {
		st.dirty = false
		st.timer = time.AfterFunc(p.debounce, func() { p.jalankan(userID) })
	}
}
