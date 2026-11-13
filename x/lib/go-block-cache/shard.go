package go_block_cache

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

var (
	defaultShardNum   = 4 * runtime.GOMAXPROCS(0) // 4 shards per cpu core
	initialBucketSize = 1 << 4
	defaultCacheSize  = 2 * MiB
	defaultCacheType  = LRU
)

type op byte

const (
	opSet op = iota
	opGet
	opDel
)

type shard struct {
	mu sync.RWMutex

	// options
	maxSize   int64
	cacheType CacheType

	cacher ICacher
	stats  Stats

	closed bool
	// points to state. As in the state don't have mutex
	state unsafe.Pointer
}

func (s *shard) getStats() Stats {
	return s.stats
}

func (s *shard) getInUsed() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cacher.GetInUsed()
}

func (s *shard) set(fileNum, key uint64, value Value) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}

	// defer until the target bucket is initialised (aka migrate data from a frozen to a new bucket)
	// but do not require blocking all operations to the hash map during the migration
	for {
		state := (*state)(atomic.LoadPointer(&s.state))
		bucket := state.lazyLoadBucket(s.getBucketId(fileNum, key, state))
		isFrozen, node := bucket.Get(fileNum, key)
		if isFrozen {
			continue
		}

		if node == nil {
			hash := murmur32(fileNum, key)
			isFrozen, node = bucket.AddNewNode(fileNum, key, hash, s)
		}
		if isFrozen {
			continue
		}

		if value == nil || computeSize(value) == 0 {
			s.evict(node)
			s.mu.Unlock()
			return true
		}

		valSize := int64(computeSize(value))
		diffSize := valSize - node.size
		node.SetValue(value, valSize)
		atomic.StoreInt32(&node.ref, 0)
		atomic.AddInt64(&s.stats.statSet, 1)
		s.mu.Unlock()

		if s.cacher != nil {
			if ok := s.cacher.Promote(node, diffSize, opSet); !ok {
				return false
			}
		}

		break
	}

	return true
}

func (s *shard) get(fileNum, key uint64) (LazyValue, bool) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, false
	}
	var isFrozen bool
	var node *kv
	// defer until the target bucket is initialised (aka migrate data from a frozen to a new bucket)
	// but do not require blocking all operations to the hash map during the migration
	for {
		state := (*state)(atomic.LoadPointer(&s.state))
		bucket := state.lazyLoadBucket(s.getBucketId(fileNum, key, state))
		isFrozen, node = bucket.Get(fileNum, key)
		if isFrozen {
			continue
		}
		if node == nil {
			atomic.AddInt64(&s.stats.statMiss, 1)
			s.mu.RUnlock()
			return nil, false
		}

		if computeSize(node.value) == 0 {
			atomic.AddInt64(&s.stats.statMiss, 1)
			s.mu.RUnlock()
			return nil, false
		}

		atomic.AddInt64(&s.stats.statHit, 1)
		node.upRef()

		s.mu.RUnlock()
		if s.cacher != nil {
			if ok := s.cacher.Promote(node, 0, opGet); !ok {
				return nil, false
			}
		}
		break
	}

	return node.ToLazyValue(), true
}

func (s *shard) delete(fileNum, key uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	var isFrozen bool
	var node *kv
	// defer until the target bucket is initialised (aka migrate data from a frozen to a new bucket)
	// but do not require blocking all operations to the hash map during the migration
	for {
		state := (*state)(atomic.LoadPointer(&s.state))
		bucket := state.lazyLoadBucket(s.getBucketId(fileNum, key, state))
		isFrozen, node = bucket.Get(fileNum, key)
		if isFrozen {
			continue
		}
		if node == nil {
			return false
		}

		s.evict(node)
		break
	}

	return true
}

// evict removes a node entirely from a hashmap
// Important: caller must ensure the lock of the hashmap
func (s *shard) evict(node *kv) bool {
	if s.closed {
		return false
	}
	atomic.StoreInt32(&node.ref, 0)
	// defer until the bucket is initialised
	var removed, isFrozen bool
	for {
		state := (*state)(atomic.LoadPointer(&s.state))
		bucket := state.lazyLoadBucket(s.getBucketId(node.fileNum, node.key, state))
		isFrozen, removed = bucket.DeleteNode(node.fileNum, node.key, node.hash, s)
		if isFrozen {
			continue
		}

		if removed {
			s.cacher.Evict(node)
			node.value = nil
			atomic.AddInt64(&s.stats.statDel, 1)
		}

		break
	}

	return removed
}

func (s *shard) getBucketId(fileNum, key uint64, state *state) uint32 {
	hash := murmur32(fileNum, key)
	return hash & state.bucketMark
}

func (s *shard) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	var allKVs []*kv
	state := (*state)(atomic.LoadPointer(&s.state))
	for i := range state.buckets {
		bucket := state.lazyLoadBucket(uint32(i))
		bucket.mu.Lock()
		allKVs = append(allKVs, bucket.nodes...)
		bucket.mu.Unlock()
	}

	for _, kv := range allKVs {
		s.evict(kv)
	}

	s.closed = true
	atomic.StorePointer(&s.state, nil)
}

func (s *shard) setCapacity(capacity int64) {
	if s.closed {
		return
	}
	if s.cacher != nil {
		s.cacher.SetCapacity(capacity)
	}
}

func newShard(cacheSize int64, cacheType CacheType) *shard {
	state := &state{
		buckets:       make([]bucket, initialBucketSize),
		bucketMark:    uint32(initialBucketSize - 1),
		growThreshold: int64(initialBucketSize * overflowThreshold),
	}
	for i := range state.buckets {
		state.buckets[i].state = initialized
	}

	c := &shard{
		state:     unsafe.Pointer(state),
		maxSize:   cacheSize,
		cacheType: cacheType,
	}

	switch cacheType {
	case LRU:
		c.cacher = newLRU(cacheSize)
	case ClockPro:
		c.cacher = NewClockPro(cacheSize)
	default:
		msg := "unsupported cache type"
		zap.L().Error(msg)
		panic(msg)
	}

	return c
}
