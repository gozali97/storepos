// Package ratelimit membatasi percobaan sensitif (login, PIN, OTP) dengan
// token bucket per kunci. Implementasi in-memory cukup untuk satu instance;
// saat horizontal scaling, pindahkan ke Redis atau tabel PostgreSQL (§16.5).
//
// Batas yang dirujuk PRD:
//   - NFR-011: login 10/menit per IP DAN per akun
//   - CORE-006: PIN terkunci 15 menit setelah 5 kali gagal
package ratelimit

import (
	"sync"
	"time"
)

// Limiter adalah token bucket per kunci.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	capacity int
	refill   time.Duration // waktu untuk mengisi 1 token
	now      func() time.Time
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// New membuat limiter dengan `events` kejadian per `per` durasi, kapasitas burst = events.
func New(events int, per time.Duration) *Limiter {
	if events <= 0 {
		events = 1
	}
	if per <= 0 {
		per = time.Minute
	}
	return &Limiter{
		buckets:  make(map[string]*bucket),
		capacity: events,
		refill:   per / time.Duration(events),
		now:      time.Now,
	}
}

// NewWithClock membuat limiter dengan sumber waktu kustom (untuk pengujian).
func NewWithClock(events int, per time.Duration, now func() time.Time) *Limiter {
	l := New(events, per)
	l.now = now
	return l
}

// Allow mengonsumsi satu token untuk kunci. Mengembalikan retryAfter > 0 bila ditolak.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(l.capacity), lastRefill: now}
		l.buckets[key] = b
	}

	// Isi ulang berdasarkan waktu yang berlalu.
	elapsed := now.Sub(b.lastRefill)
	if elapsed > 0 {
		b.tokens += float64(elapsed) / float64(l.refill)
		if b.tokens > float64(l.capacity) {
			b.tokens = float64(l.capacity)
		}
		b.lastRefill = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	// Waktu tunggu sampai satu token tersedia.
	need := 1 - b.tokens
	wait := time.Duration(need * float64(l.refill))
	return false, wait
}

// Reset menghapus riwayat satu kunci (mis. setelah login berhasil).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Prune menghapus bucket yang sudah penuh dan tidak aktif, untuk mencegah
// pertumbuhan memori tanpa batas. Panggil berkala dari worker.
func (l *Limiter) Prune() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	idle := time.Duration(l.capacity) * l.refill * 2
	removed := 0
	for k, b := range l.buckets {
		if now.Sub(b.lastRefill) > idle {
			delete(l.buckets, k)
			removed++
		}
	}
	return removed
}

// Size melaporkan jumlah kunci yang dilacak (untuk metrik).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
