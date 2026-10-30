package ratelimit

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	store := NewMemoryStore(1*time.Minute, 100)
	defer store.Close()

	key := "user:1"
	limit := 3
	window := 1 * time.Second

	// 3 request pertama harus diizinkan
	for i := 0; i < limit; i++ {
		allowed, rem, _ := store.Allow(key, limit, window)
		if !allowed {
			t.Fatalf("request %d seharusnya diizinkan", i+1)
		}
		if rem != limit-1-i {
			t.Errorf("sisa token diharapkan %d, dapat %d", limit-1-i, rem)
		}
	}

	// Request ke-4 harus ditolak (429)
	allowed, _, retryAfter := store.Allow(key, limit, window)
	if allowed {
		t.Fatal("request ke-4 harusnya ditolak")
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter diharapkan > 0, dapat %v", retryAfter)
	}

	// Tunggu refill
	time.Sleep(window + 100*time.Millisecond)
	allowed, _, _ = store.Allow(key, limit, window)
	if !allowed {
		t.Fatal("request setelah window seharusnya diizinkan kembali")
	}
}

func TestLimiter_Concurrency(t *testing.T) {
	store := NewMemoryStore(1*time.Minute, 1000)
	defer store.Close()

	key := "concurrent:user"
	limit := 20
	window := 1 * time.Second
	routines := 50

	var wg sync.WaitGroup
	var allowedCount int32
	var rejectedCount int32

	wg.Add(routines)
	for i := 0; i < routines; i++ {
		go func() {
			defer wg.Done()
			allowed, _, _ := store.Allow(key, limit, window)
			if allowed {
				atomic.AddInt32(&allowedCount, 1)
			} else {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}
	wg.Wait()

	if allowedCount != int32(limit) {
		t.Errorf("jumlah request diizinkan diharapkan %d, dapat %d", limit, allowedCount)
	}
	if rejectedCount != int32(routines-limit) {
		t.Errorf("jumlah request ditolak diharapkan %d, dapat %d", routines-limit, rejectedCount)
	}
}

func TestLimiter_MaxEntriesEviction(t *testing.T) {
	maxEntries := 10
	store := NewMemoryStore(1*time.Minute, maxEntries)
	defer store.Close()

	// Tambahkan 50 key berbeda
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("ip:%d", i)
		store.Allow(key, 5, 10*time.Millisecond)
	}

	store.mu.Lock()
	count := len(store.buckets)
	store.mu.Unlock()

	if count > maxEntries {
		t.Errorf("jumlah key (%d) melebihi maxEntries (%d)", count, maxEntries)
	}
}

func TestLimiter_ThrottledBucketProtectionFromEviction(t *testing.T) {
	maxEntries := 5
	store := NewMemoryStore(1*time.Minute, maxEntries)
	defer store.Close()

	abuserKey := "ip:abuser"
	limit := 2
	window := 5 * time.Second

	// Habiskan kuota abuser
	store.Allow(abuserKey, limit, window)
	store.Allow(abuserKey, limit, window)
	allowed, _, _ := store.Allow(abuserKey, limit, window)
	if allowed {
		t.Fatal("abuser harusnya sudah throttled")
	}

	// Semprot dengan 30 IP baru untuk memicu eviction berulang kali
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("ip:random_%d", i)
		store.Allow(key, 5, window)
	}

	// Abuser harus tetap throttled (tidak boleh ter-reset kuotanya akibat semprotan IP)
	allowed, _, _ = store.Allow(abuserKey, limit, window)
	if allowed {
		t.Fatal("abuser TIDAK BOLEH di-reset kuotanya akibat semprotan IP baru")
	}
}
