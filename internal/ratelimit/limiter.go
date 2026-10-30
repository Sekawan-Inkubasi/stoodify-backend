package ratelimit

import (
	"container/list"
	"math"
	"sync"
	"time"
)

// LimiterStore mendefinisikan antarmuka penyimpan rate limit
type LimiterStore interface {
	Allow(key string, limit int, window time.Duration) (allowed bool, remaining int, retryAfter time.Duration)
	Close()
}

type bucketEntry struct {
	key         string
	tokens      float64
	lastUpdated time.Time
	elem        *list.Element
}

// MemoryStore adalah implementasi in-memory token bucket dengan indeks LRU O(1)
type MemoryStore struct {
	mu         sync.Mutex
	buckets    map[string]*bucketEntry
	lruList    *list.List
	maxEntries int
	stopChan   chan struct{}
	closed     bool
}

// NewMemoryStore membuat instance MemoryStore baru dengan interval cleanup dan batas entri
func NewMemoryStore(cleanupInterval time.Duration, maxEntries int) *MemoryStore {
	if maxEntries <= 0 {
		maxEntries = 50000
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 2 * time.Minute
	}

	s := &MemoryStore{
		buckets:    make(map[string]*bucketEntry),
		lruList:    list.New(),
		maxEntries: maxEntries,
		stopChan:   make(chan struct{}),
	}

	go s.cleanupLoop(cleanupInterval)
	return s
}

func (s *MemoryStore) Allow(key string, limit int, window time.Duration) (bool, int, time.Duration) {
	if limit <= 0 || window <= 0 {
		return true, limit, 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	b, exists := s.buckets[key]
	rate := float64(limit) / window.Seconds()

	if !exists {
		// Jika kapasitas penuh, lakukan eviction O(1) berbasis ekor LRU
		if len(s.buckets) >= s.maxEntries {
			s.evictOldestLocked(now, window)
		}

		// Jika tabel tetap penuh karena seluruh kandidat LRU sedang aktif throttled,
		// tolak alokasi baru demi keamanan memori dan menjaga kuota pelanggar.
		if len(s.buckets) >= s.maxEntries {
			return false, 0, window
		}

		entry := &bucketEntry{
			key:         key,
			tokens:      float64(limit) - 1.0,
			lastUpdated: now,
		}
		entry.elem = s.lruList.PushFront(entry)
		s.buckets[key] = entry
		return true, int(entry.tokens), 0
	}

	// Update posisi ke kepala LRU O(1)
	s.lruList.MoveToFront(b.elem)

	// Hitung token yang terisi kembali berdasarkan waktu yang telah lewat
	elapsed := now.Sub(b.lastUpdated).Seconds()
	b.tokens = math.Min(float64(limit), b.tokens+(elapsed*rate))
	b.lastUpdated = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, int(b.tokens), 0
	}

	// Kuota habis, hitung sisa waktu tunggu (Retry-After)
	needed := 1.0 - b.tokens
	waitSeconds := needed / rate
	retryAfter := time.Duration(math.Ceil(waitSeconds)) * time.Second
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return false, 0, retryAfter
}

// evictOldestLocked membersihkan entri dari ekor LRU secara O(1) tanpa memindai seluruh map
func (s *MemoryStore) evictOldestLocked(now time.Time, window time.Duration) {
	checked := 0
	elem := s.lruList.Back()
	for elem != nil && len(s.buckets) >= s.maxEntries && checked < 50 {
		prev := elem.Prev()
		entry := elem.Value.(*bucketEntry)
		isExpired := now.Sub(entry.lastUpdated) > window
		isThrottled := entry.tokens < 1.0 && now.Sub(entry.lastUpdated) <= window

		// Hapus jika sudah kedaluwarsa atau tidak sedang dalam penalti aktif (throttled)
		if isExpired || !isThrottled {
			s.lruList.Remove(elem)
			delete(s.buckets, entry.key)
		}
		elem = prev
		checked++
	}
}

func (s *MemoryStore) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case now := <-ticker.C:
			s.mu.Lock()
			// Background sweep bertahap untuk membersihkan entri yang idle lebih dari 10 menit
			count := 0
			for elem := s.lruList.Back(); elem != nil && count < 1000; {
				prev := elem.Prev()
				entry := elem.Value.(*bucketEntry)
				if now.Sub(entry.lastUpdated) > 10*time.Minute {
					s.lruList.Remove(elem)
					delete(s.buckets, entry.key)
				}
				elem = prev
				count++
			}
			s.mu.Unlock()
		}
	}
}

func (s *MemoryStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.stopChan)
	}
}
