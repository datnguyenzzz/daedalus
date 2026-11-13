package go_block_cache

import (
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/stretchr/testify/require"
)

var valueSize = int(1 * KiB)

// Ristretto V2

// Sync

func Benchmark_Ristretto_Cache_Add_Read(b *testing.B) {
	var m0, m1 runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m0)

	cache, err := ristretto.NewCache(&ristretto.Config[uint64, []byte]{
		NumCounters: 100_000_000, // 5x estimated nodes
		MaxCost:     100 * KiB,
		BufferItems: 64,
		Metrics:     true,
	})
	require.NoError(b, err)
	if err != nil {
		panic(err)
	}

	cnt := 0
	for b.Loop() {
		cnt += 1
		k, v := uint64(cnt), randomBytes(valueSize)
		_ = cache.Set(k, v, int64(len(v)))
		_, _ = cache.Get(max(1, uint64(rand.Intn(101)+cnt-100)))
	}

	runtime.GC()
	runtime.ReadMemStats(&m1)

	totalBytes := m1.TotalAlloc - m0.TotalAlloc

	b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
	b.ReportMetric(float64(100*KiB-cache.RemainingCost()), "mem_footprint_in_bytes")
	b.ReportMetric(cache.Metrics.Ratio(), "hit_ratio")
}

// ASync

func Benchmark_Ristretto_Cache_Read_After_Write_Async(b *testing.B) {
	var m0, m1 runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m0)

	cache, err := ristretto.NewCache(&ristretto.Config[uint64, []byte]{
		NumCounters: 100_000_000, // 5x estimated nodes
		MaxCost:     100 * KiB,
		BufferItems: 64,
		Metrics:     true,
	})
	require.NoError(b, err)
	if err != nil {
		panic(err)
	}
	cnt := int64(0)
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&cnt, 1)
			k, v := uint64(cnt), randomBytes(valueSize)
			_ = cache.Set(k, v, int64(len(v)))
			_, _ = cache.Get(k)
		}
	})

	runtime.GC()
	runtime.ReadMemStats(&m1)

	totalBytes := m1.TotalAlloc - m0.TotalAlloc

	b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
	b.ReportMetric(float64(100*KiB-cache.RemainingCost()), "mem_footprint_in_bytes")
	b.ReportMetric(cache.Metrics.Ratio(), "hit_ratio")
}

func Benchmark_Ristretto_Cache_Add_Read_Random_Async(b *testing.B) {
	var m0, m1 runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m0)

	cache, err := ristretto.NewCache(&ristretto.Config[uint64, []byte]{
		NumCounters: 100_000_000, // 5x estimated nodes
		MaxCost:     100 * KiB,
		BufferItems: 64,
		Metrics:     true,
	})
	require.NoError(b, err)
	if err != nil {
		panic(err)
	}
	cnt := int64(0)
	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&cnt, 1)
			k, v := uint64(cnt), randomBytes(valueSize)
			_ = cache.Set(k, v, int64(len(v)))
			_, _ = cache.Get(max(1, uint64(rand.Int63n(101)+int64(k)-100)))
		}
	})

	runtime.GC()
	runtime.ReadMemStats(&m1)

	totalBytes := m1.TotalAlloc - m0.TotalAlloc

	b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
	b.ReportMetric(float64(100*KiB-cache.RemainingCost()), "mem_footprint_in_bytes")
	b.ReportMetric(cache.Metrics.Ratio(), "hit_ratio")
}

// NogoDB LRU - block-cache

// Sync

func Benchmark_NogoDB_Cache_Add_Read(b *testing.B) {
	cachePolicies := []CacheType{
		LRU,
		ClockPro,
	}
	for _, cp := range cachePolicies {
		b.Run(cp.toString(), func(b *testing.B) {
			var m0, m1 runtime.MemStats

			runtime.GC()
			runtime.ReadMemStats(&m0)

			c := NewMap(
				WithCacheType(LRU),
				WithMaxSize(100*KiB),
			)

			cnt := 0
			for b.Loop() {
				cnt += 1
				k, v := uint64(cnt), randomBytes(valueSize)
				_ = c.Set(0, k, v)
				_, _ = c.Get(0, max(1, uint64(rand.Intn(101)+cnt-100)))
			}
			runtime.GC()
			runtime.ReadMemStats(&m1)

			totalBytes := m1.TotalAlloc - m0.TotalAlloc

			b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
			b.ReportMetric(float64(c.GetInUsed()), "mem_footprint_in_bytes")
			b.ReportMetric(float64(c.GetStats().statHit)/float64(c.GetStats().statHit+c.GetStats().statMiss), "hit_ratio")
		})
	}
}

// ASync

func Benchmark_NogoDB_Cache_Read_After_Write_Async(b *testing.B) {
	cachePolicies := []CacheType{
		LRU,
		ClockPro,
	}
	for _, cp := range cachePolicies {
		b.Run(cp.toString(), func(b *testing.B) {
			var m0, m1 runtime.MemStats

			runtime.GC()
			runtime.ReadMemStats(&m0)
			c := NewMap(
				WithCacheType(LRU),
				WithMaxSize(100*KiB),
			)

			cnt := int64(0)
			b.SetParallelism(20)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					atomic.AddInt64(&cnt, 1)
					k, v := uint64(cnt), randomBytes(valueSize)
					_ = c.Set(0, k, v)
					_, _ = c.Get(0, k)
				}
			})
			runtime.GC()
			runtime.ReadMemStats(&m1)

			totalBytes := m1.TotalAlloc - m0.TotalAlloc

			b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
			b.ReportMetric(float64(c.GetInUsed()), "mem_footprint_in_bytes")
			b.ReportMetric(float64(c.GetStats().statHit)/float64(c.GetStats().statHit+c.GetStats().statMiss), "hit_ratio")
		})
	}
}

func Benchmark_NogoDB_Cache_Add_Read_Random_Async(b *testing.B) {
	cachePolicies := []CacheType{
		LRU,
		ClockPro,
	}
	for _, cp := range cachePolicies {
		b.Run(cp.toString(), func(b *testing.B) {
			var m0, m1 runtime.MemStats

			runtime.GC()
			runtime.ReadMemStats(&m0)
			c := NewMap(
				WithCacheType(LRU),
				WithMaxSize(100*KiB),
			)

			cnt := int64(0)
			b.SetParallelism(20)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					atomic.AddInt64(&cnt, 1)
					k, v := uint64(cnt), randomBytes(valueSize)
					_ = c.Set(0, k, v)
					_, _ = c.Get(0, max(1, uint64(rand.Int63n(101)+int64(k)-100)))
				}
			})
			runtime.GC()
			runtime.ReadMemStats(&m1)

			totalBytes := m1.TotalAlloc - m0.TotalAlloc

			b.ReportMetric(float64(totalBytes), "total_bytes_allocated")
			b.ReportMetric(float64(c.GetInUsed()), "mem_footprint_in_bytes")
			b.ReportMetric(float64(c.GetStats().statHit)/float64(c.GetStats().statHit+c.GetStats().statMiss), "hit_ratio")
		})
	}
}

func randomBytes(sz int) []byte {
	res := make([]byte, sz)
	for i := range sz {
		res[i] = byte(rand.Intn(256))
	}
	return res
}
