package common

import (
	"fmt"

	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
)

type ValueSource byte

const (
	ValueFromUnknown ValueSource = iota
	ValueFromBuffer
	ValueFromCache
)

// Fetcher uses to fetch and/or release the data
type Fetcher interface {
	Load() []byte
	// Release freed associated resources. Release should always success
	// and can be called multiple times without causing error.
	Release()
}

type BufferPoolFetcher struct {
	pool *predictable_size.PredictablePool
	val  []byte

	Fetcher
}

func (b *BufferPoolFetcher) Reserve(size int) {
	b.val = b.pool.Get(size)
	b.val = b.val[:size]
}

func (b *BufferPoolFetcher) Set(val []byte) {
	if len(b.val) != len(val) {
		b.Reserve(len(val))
	}
	copy(b.val, val)
}

func (b *BufferPoolFetcher) Load() []byte {
	return b.val
}

func (b *BufferPoolFetcher) Release() {
	b.pool.Put(b.val)
	b.val = nil
}

// InternalLazyValue either points to a block in the block cache (Fetcher != nil),
// or a buffer that exists outside the block cache allocated from a BufferPool.
//
// The value of the InternalLazyValue might not yet dereference, until caller
// explicitly load the value.
type InternalLazyValue struct {
	ValueSource   ValueSource
	BufferFetcher *BufferPoolFetcher
	CacheFetcher  Fetcher
}

func NewBlankInternalLazyValue(s ValueSource) InternalLazyValue {
	return InternalLazyValue{
		ValueSource: s,
	}
}

type InternalKV struct {
	K InternalKey
	V InternalLazyValue
}

func (kv *InternalKV) Compare(comparer IComparer, another *InternalKV) int {
	if c := kv.K.Compare(comparer, &another.K); c != 0 {
		return c
	}

	return kv.V.Compare(comparer, &another.V)
}

// Value loads value
func (iv *InternalLazyValue) Value() []byte {
	switch iv.ValueSource {
	case ValueFromBuffer:
		return iv.BufferFetcher.Load()
	case ValueFromCache:
		return iv.CacheFetcher.Load()
	default:
		panic(fmt.Sprintf("InternalLazyValue.Value: unknown value source: %d", iv.ValueSource))
	}
}

// Release releases the allocated memory to store the value
func (iv *InternalLazyValue) Release() {
	switch iv.ValueSource {
	case ValueFromBuffer:
		iv.BufferFetcher.Release()
	case ValueFromCache:
		iv.CacheFetcher.Release()
	default:
		panic(fmt.Sprintf("InternalLazyValue.Value: unknown value source: %d", iv.ValueSource))
	}
}

// ReserveBuffer uses for reserving a define allocated memory from a buffer pool
func (iv *InternalLazyValue) ReserveBuffer(pool *predictable_size.PredictablePool, size int) {
	iv.ValueSource = ValueFromBuffer
	iv.BufferFetcher = &BufferPoolFetcher{
		pool: pool,
	}
	iv.BufferFetcher.Reserve(size)
}

// SetBufferValue used for manually update the buffered value of the BufferFetcher
// A caller must use the Reserve function before using this function to load the data into
// the reserved buffer
func (iv *InternalLazyValue) SetBufferValue(value []byte) error {
	if iv.ValueSource != ValueFromBuffer {
		return fmt.Errorf("value source: %d has not supported this function", iv.ValueSource)
	}
	if len(value) != len(iv.BufferFetcher.Load()) {
		return fmt.Errorf("the buffered capacity is different")
	}

	iv.BufferFetcher.Set(value)
	return nil
}

func (iv *InternalLazyValue) SetCacheFetcher(fetcher Fetcher) error {
	if iv.ValueSource != ValueFromCache {
		return fmt.Errorf("value source: %d has not supported this function", iv.ValueSource)
	}
	iv.CacheFetcher = fetcher
	return nil
}

func (iv *InternalLazyValue) Compare(comparer IComparer, lv *InternalLazyValue) int {
	return comparer.Compare(iv.Value(), lv.Value())
}
