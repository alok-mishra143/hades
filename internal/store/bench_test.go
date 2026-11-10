package store

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// Benchmarks for the store engine. Run them without -race, which is the normal
// way to measure throughput:
//
//	go test ./internal/store/ -run '^$' -bench . -benchmem -count=5 -benchtime=1s
//
// Every key string is generated once at package initialization, outside any
// timed loop, so key formatting never shows up in the measurements. The
// sequential benchmarks use a 10,000-key working set; the 100,000-key wide set
// feeds the large-sweep and parallel-write benchmarks.

const (
	benchKeyCount  = 10_000
	benchWideCount = 100_000
	benchValue     = "bench-value"
)

func makeBenchKeys(prefix string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s%05d", prefix, i)
	}
	return keys
}

var (
	// benchHitKeys is the 10k-key working set for the sequential benchmarks.
	benchHitKeys = makeBenchKeys("key:", benchKeyCount)
	// benchMissKeys is disjoint from benchHitKeys, so stores preloaded from
	// benchHitKeys never contain these keys.
	benchMissKeys = makeBenchKeys("miss:", benchKeyCount)
	// benchWideKeys is the 100k-key space for the sweep and parallel-write
	// benchmarks.
	benchWideKeys = makeBenchKeys("wide:", benchWideCount)

	// benchWorkerSeq hands each parallel worker a different starting offset.
	benchWorkerSeq uint64
)

// benchWorkerOffset returns a per-worker starting index into a slice of n keys.
// The prime stride keeps workers from walking the same keys in lockstep.
func benchWorkerOffset(n int) int {
	w := atomic.AddUint64(&benchWorkerSeq, 1)
	return int((w * 7919) % uint64(n))
}

// BenchmarkStore_Set models SET on a warm keyspace. The first pass over the 10k
// keys inserts new entries; every later pass overwrites an existing key, which
// is the common case for a cache.
func BenchmarkStore_Set(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	i := 0
	for b.Loop() {
		if err := s.Set(benchHitKeys[i], benchValue); err != nil {
			b.Fatal(err)
		}
		i++
		if i == len(benchHitKeys) {
			i = 0
		}
	}
}

// BenchmarkStore_GetHit models the hot read path: the key exists, so Get takes
// the lock, checks expiry, and returns the stored value.
func BenchmarkStore_GetHit(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	for _, key := range benchHitKeys {
		if err := s.Set(key, benchValue); err != nil {
			b.Fatalf("preload %q: %v", key, err)
		}
	}

	i := 0
	for b.Loop() {
		if _, ok := s.Get(benchHitKeys[i]); !ok {
			b.Fatalf("Get(%q) missed", benchHitKeys[i])
		}
		i++
		if i == len(benchHitKeys) {
			i = 0
		}
	}
}

// BenchmarkStore_GetMiss models a negative lookup. The store holds 10k live
// entries, so map bucket density matches GetHit; the miss returns before the
// expiry clock is read.
func BenchmarkStore_GetMiss(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	for _, key := range benchHitKeys {
		if err := s.Set(key, benchValue); err != nil {
			b.Fatalf("preload %q: %v", key, err)
		}
	}

	i := 0
	for b.Loop() {
		if _, ok := s.Get(benchMissKeys[i]); ok {
			b.Fatalf("Get(%q) unexpectedly hit", benchMissKeys[i])
		}
		i++
		if i == len(benchMissKeys) {
			i = 0
		}
	}
}

// BenchmarkStore_SetGet models a SET immediately followed by a GET on the same
// key, e.g. a write-then-read request pair. It pays for two lock acquisitions
// per logical operation.
func BenchmarkStore_SetGet(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	i := 0
	for b.Loop() {
		key := benchHitKeys[i]
		if err := s.Set(key, benchValue); err != nil {
			b.Fatal(err)
		}
		if _, ok := s.Get(key); !ok {
			b.Fatalf("Get(%q) missed after Set", key)
		}
		i++
		if i == len(benchHitKeys) {
			i = 0
		}
	}
}

// BenchmarkStore_SetWithTTL models SET with an expiry. The TTL is one hour, so
// nothing can expire during a one-second run; the benchmark measures the
// expiry-aware write path, not expiration behavior.
func BenchmarkStore_SetWithTTL(b *testing.B) {
	b.ReportAllocs()
	const ttl = time.Hour

	s := NewStore()
	i := 0
	for b.Loop() {
		if err := s.SetWithTTL(benchHitKeys[i], benchValue, ttl); err != nil {
			b.Fatal(err)
		}
		i++
		if i == len(benchHitKeys) {
			i = 0
		}
	}
}

// BenchmarkStore_ParallelGet models concurrent GET traffic from many clients
// against a warm 10k-key store. Workers start at different offsets so they do
// not read the same key in lockstep. The plain sync.Mutex is the throughput
// ceiling here.
func BenchmarkStore_ParallelGet(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	for _, key := range benchHitKeys {
		if err := s.Set(key, benchValue); err != nil {
			b.Fatalf("preload %q: %v", key, err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := benchWorkerOffset(len(benchHitKeys))
		for pb.Next() {
			if _, ok := s.Get(benchHitKeys[i]); !ok {
				b.Errorf("Get(%q) missed", benchHitKeys[i])
				return
			}
			i++
			if i == len(benchHitKeys) {
				i = 0
			}
		}
	})
}

// BenchmarkStore_ParallelSet models concurrent SET traffic. Each worker walks
// its own offset through a 100k-key space, so writes spread across many keys
// instead of overwriting one hot key. The map fills to 100k entries and later
// iterations recycle that working set as overwrites.
func BenchmarkStore_ParallelSet(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := benchWorkerOffset(len(benchWideKeys))
		for pb.Next() {
			if err := s.Set(benchWideKeys[i], benchValue); err != nil {
				b.Errorf("Set: %v", err)
				return
			}
			i++
			if i == len(benchWideKeys) {
				i = 0
			}
		}
	})
}

// BenchmarkStore_SweepLive100k models active expiry over a large keyspace where
// nothing has expired: Sweep visits all 100k entries and deletes none. This is
// the worst case for the O(n) sweeper and the number that matters until an
// indexed expiry structure exists.
func BenchmarkStore_SweepLive100k(b *testing.B) {
	b.ReportAllocs()
	s := NewStore()
	for _, key := range benchWideKeys {
		if err := s.Set(key, benchValue); err != nil {
			b.Fatalf("preload %q: %v", key, err)
		}
	}

	for b.Loop() {
		if removed := s.Sweep(); removed != 0 {
			b.Fatalf("Sweep() = %d, want 0", removed)
		}
	}
}

// BenchmarkStore_SetFull models a write rejected by the capacity limit. The
// store sits exactly at maxSize with live entries, so each Set misses, scans
// all 10k entries looking for expired keys, finds none, and returns ErrOOM.
// This exposes the O(n) sweep on the capacity path: a rejected write is far
// more expensive than an accepted one.
func BenchmarkStore_SetFull(b *testing.B) {
	b.ReportAllocs()
	const maxSize = benchKeyCount

	s := NewStore(maxSize)
	for _, key := range benchHitKeys {
		if err := s.Set(key, benchValue); err != nil {
			b.Fatalf("preload %q: %v", key, err)
		}
	}

	i := 0
	for b.Loop() {
		err := s.Set(benchMissKeys[i], benchValue)
		if !errors.Is(err, ErrOOM) {
			b.Fatalf("Set on full store = %v, want %v", err, ErrOOM)
		}
		i++
		if i == len(benchMissKeys) {
			i = 0
		}
	}
}
