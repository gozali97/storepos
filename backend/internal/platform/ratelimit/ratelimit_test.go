package ratelimit

import (
	"testing"
	"time"
)

func TestAllowConsumesUpToCapacity(t *testing.T) {
	var now = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewWithClock(10, time.Minute, clock)

	for i := 1; i <= 10; i++ {
		ok, _ := l.Allow("ip:1.2.3.4")
		if !ok {
			t.Fatalf("percobaan ke-%d harus diizinkan", i)
		}
	}

	ok, retry := l.Allow("ip:1.2.3.4")
	if ok {
		t.Fatal("percobaan ke-11 harus ditolak")
	}
	if retry <= 0 {
		t.Fatalf("retryAfter harus positif, dapat %v", retry)
	}
}

func TestRefillsOverTime(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewWithClock(10, time.Minute, clock)

	for i := 0; i < 10; i++ {
		l.Allow("akun:a@b.c")
	}
	if ok, _ := l.Allow("akun:a@b.c"); ok {
		t.Fatal("harus ditolak saat habis")
	}

	// 6 detik = 1 token pada laju 10/menit.
	now = now.Add(6 * time.Second)
	if ok, _ := l.Allow("akun:a@b.c"); !ok {
		t.Fatal("harus diizinkan setelah 6 detik")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewWithClock(3, time.Minute, func() time.Time { return now })

	for i := 0; i < 3; i++ {
		l.Allow("ip:1.1.1.1")
	}
	if ok, _ := l.Allow("ip:1.1.1.1"); ok {
		t.Fatal("ip A harus habis")
	}
	if ok, _ := l.Allow("ip:2.2.2.2"); !ok {
		t.Fatal("ip B tidak boleh terpengaruh ip A")
	}
}

func TestReset(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewWithClock(2, time.Minute, func() time.Time { return now })

	l.Allow("k")
	l.Allow("k")
	if ok, _ := l.Allow("k"); ok {
		t.Fatal("harus habis")
	}
	l.Reset("k")
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("setelah reset harus penuh lagi")
	}
}

func TestPruneRemovesIdleBuckets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := NewWithClock(10, time.Minute, clock)

	l.Allow("lama")
	l.Allow("baru")
	if l.Size() != 2 {
		t.Fatalf("harus 2 bucket, dapat %d", l.Size())
	}

	now = now.Add(1 * time.Hour)
	l.Allow("baru") // sentuh bucket "baru"
	if removed := l.Prune(); removed != 1 {
		t.Fatalf("harus menghapus 1 bucket, dapat %d", removed)
	}
	if l.Size() != 1 {
		t.Fatalf("sisa harus 1 bucket, dapat %d", l.Size())
	}
}

func TestZeroValuesAreSafe(t *testing.T) {
	l := New(0, 0)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("limiter dengan nilai nol harus tetap mengizinkan sekali")
	}
}

func TestConcurrentAccess(t *testing.T) {
	l := New(1000, time.Minute)
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				l.Allow("bersama")
			}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// Hanya memastikan tidak ada data race / panic; -race akan menangkapnya.
}
