package go_bytesbufferpool

import (
	"bytes"
	"sync"
	"testing"

	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
)

var bufs = [][]byte{ // Total: 26MB
	makeDummyBuffer(1 * 1024 * 1024), // 1 MB
	makeDummyBuffer(4 * 1024 * 1024), // 4 MB
	makeDummyBuffer(2 * 1024 * 1024), // 2 MB
	makeDummyBuffer(8 * 1024 * 1024), // 8 MB
	makeDummyBuffer(8 * 1024 * 1024), // 8 MB
	makeDummyBuffer(2 * 1024 * 1024), // 2 MB
	makeDummyBuffer(4 * 1024 * 1024), // 4 MB
	makeDummyBuffer(1 * 1024 * 1024), // 1 MB
}

func makeDummyBuffer(size int) []byte {
	buf := make([]byte, size)
	for i := range size {
		buf[i] = 0xff
	}
	return buf
}

func Benchmark_SyncPool_Buffer(b *testing.B) {
	for b.Loop() {
		sPool := sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		}
		for _, b := range bufs {
			// get from the pool
			buf := sPool.Get().(*bytes.Buffer)
			if buf.Cap() < len(b) {
				buf.Grow(len(b))
			}
			_, _ = buf.Write(b)
			// put back to the pool
			buf.Reset()
			sPool.Put(buf)
		}
	}
}

func Benchmark_Predictable_Size_Buffer(b *testing.B) {
	for b.Loop() {
		pool := predictable_size.NewPredictablePool()
		for _, b := range bufs {
			buf := pool.Get(len(b))
			buf = append(buf, b...)
			pool.Put(buf)
		}
	}
}
