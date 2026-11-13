package predictable_size

import (
	"math"
	"runtime"
	"sync"
	"testing"
)

func TestGetPoolIDAndCapacity(t *testing.T) {
	tests := []struct {
		name         string
		size         int
		expectedID   int
		expectedCap  int
		expectedPool int
	}{
		{"zero size", 0, 0, 256, 0},
		{"one byte", 1, 0, 256, 0},
		{"max small pool", 256, 0, 256, 0},
		{"min medium pool", 257, 1, 512, 1},
		{"max medium pool", 512, 1, 512, 1},
		{"min large pool", 513, 2, 1024, 2},
		{"max large pool", 1024, 2, 1024, 2},
		{"min very large pool", 1025, 3, 2048, 3},
		{"large size", 1 << 20, 20 - 8, 1 << 20, 20 - 8},
		{"max id", math.MaxInt32, 23, 1 << 31, 23},
		{"negative size", -1, 0, 256, 0}, // Should handle negative values
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, cap := getPoolIDAndCapacity(tt.size)
			if id != tt.expectedID {
				t.Errorf("getPoolIDAndCapacity(%d) poolID = %d, want %d", tt.size, id, tt.expectedID)
			}
			if cap != tt.expectedCap {
				t.Errorf("getPoolIDAndCapacity(%d) capacity = %d, want %d", tt.size, cap, tt.expectedCap)
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		size        int
		expectedCap int
	}{
		{"zero size", 0, 256},
		{"small size", 128, 256},
		{"max small size", 256, 256},
		{"medium size", 300, 512},
		{"max medium size", 512, 512},
		{"large size", 1000, 1024},
		{"very large size", 2000, 2048},
		{"huge size", 1 << 20, 1 << 20},
		{"negative size", -10, 256}, // Should handle negative values gracefully
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPredictablePool()
			b := pool.Get(tt.size)

			// Test if the buffer has the expected capacity
			if cap(b) != tt.expectedCap {
				t.Errorf("Get(%d) cap = %d, want %d", tt.size, cap(b), tt.expectedCap)
			}

			// Test if the buffer has zero length
			if len(b) != 0 {
				t.Errorf("Get(%d) len = %d, want 0", tt.size, len(b))
			}
		})
	}
}

func TestPut(t *testing.T) {
	// Test putting valid buffers
	tests := []struct {
		name         string
		cap          int
		len          int
		shouldBeSame bool
	}{
		{"zero cap buffer", 0, 0, false},
		{"small buffer", 256, 100, true},
		{"medium buffer", 512, 300, true},
		{"large buffer", 1024, 700, true},
		{"exact capacity", 256, 256, true},
		{"oversized buffer", 2000, 1000, false}, // This should not be put back in the pool
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPredictablePool()
			// Create a buffer and put it in the pool
			buf := make([]byte, tt.len, tt.cap)
			// Write some content to verify buffer is reused
			for i := range tt.len {
				buf[i] = byte(i % 256)
			}

			// Put in pool
			pool.Put(buf)

			// Try to get the same buffer back
			newBuf := pool.Get(tt.cap)

			// If the buffer shouldn't be in the pool, we can't verify much
			if !tt.shouldBeSame {
				return
			}

			// Check if we got a buffer with expected capacity
			if cap(newBuf) != tt.cap {
				t.Errorf("Put/Get buffer capacity = %d, want %d", cap(newBuf), tt.cap)
			}

			// Check if the buffer was reset (length should be 0)
			if len(newBuf) != 0 {
				t.Errorf("Put/Get buffer length = %d, want 0", len(newBuf))
			}
		})
	}
}

func TestPoolReuse(t *testing.T) {
	// Test reusing buffers from the pool
	size := 256

	// Clear the specific pool
	pool := NewPredictablePool()

	// Get a buffer, modify it, and put it back
	buf := pool.Get(size)
	buf = append(buf, []byte("test data")...)
	pool.Put(buf)

	// Get a buffer again - should be the same one
	newBuf := pool.Get(size)
	if len(newBuf) != 0 {
		t.Errorf("Buffer not reset: len = %d, want 0", len(newBuf))
	}

	// Verify the capacity is correct
	if cap(newBuf) != 256 {
		t.Errorf("Buffer capacity = %d, want 256", cap(newBuf))
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("exact power of 2 boundaries", func(t *testing.T) {
		// Test exact power of 2 boundaries
		for i := 8; i < 16; i++ {
			size := 1 << i

			// Size exactly at power of 2
			id1, cap1 := getPoolIDAndCapacity(size)

			// Size just above power of 2
			id2, cap2 := getPoolIDAndCapacity(size + 1)

			if id1 == id2 {
				t.Errorf("Sizes %d and %d should be in different pools", size, size+1)
			}

			if cap1 == cap2 {
				t.Errorf("Sizes %d and %d should have different capacities", size, size+1)
			}
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	const workers = 8
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(workers)

	pool := NewPredictablePool()

	for i := range workers {
		go func(id int) {
			defer wg.Done()

			for j := range iterations {
				size := (id*iterations + j) % 10000
				buf := pool.Get(size)
				runtime.Gosched() // Force potential race conditions
				pool.Put(buf)
			}
		}(i)
	}

	wg.Wait()
}
