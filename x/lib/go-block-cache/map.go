package go_block_cache

import (
	"go.uber.org/zap"
)

var (
	B   = int64(1)
	KiB = 1024 * B
	MiB = 1024 * KiB
)

type Value []byte

type CacheType byte

const (
	Unknown CacheType = iota
	LRU
	ClockPro
)

func (ct CacheType) toString() string {
	switch ct {
	case LRU:
		return "LRU"
	case ClockPro:
		return "ClockPro"
	default:
		panic("not supported cache type")
	}
}

type Stats struct {
	statNodes  int64
	statGrow   int32
	statShrink int32
	statHit    int64
	statMiss   int64
	statSet    int64
	statDel    int64
}

// hashMap represent a hash map. It and its eviction policies is not a
// fully lock-free data structure, so to reduce lock contention, a
// common practice is to shard it by key into multiple shards
type hashMap struct {
	shards    []*shard
	maxSize   int64
	cacheType CacheType
	shardNum  int
}

func (h *hashMap) GetStats() Stats {
	total := Stats{}
	for _, s := range h.shards {
		stats := s.getStats()
		total.statNodes += stats.statNodes
		total.statGrow += stats.statGrow
		total.statShrink += stats.statShrink
		total.statHit += stats.statHit
		total.statMiss += stats.statMiss
		total.statSet += stats.statSet
		total.statDel += stats.statDel
	}
	return total
}

func (h *hashMap) GetInUsed() int64 {
	var total int64
	for _, s := range h.shards {
		total += s.getInUsed()
	}
	return total
}

func (h *hashMap) Set(fileNum, key uint64, value Value) bool {
	if int64(computeSize(value)) > h.maxSize/int64(h.shardNum) {
		zap.L().Error("value size exceeds the maximum cache size per shard", zap.Int64("max_cache_size_per_shard", h.maxSize/int64(h.shardNum)))
		return false
	}

	return h.getShard(fileNum, key).set(fileNum, key, value)
}

func (h *hashMap) Get(fileNum, key uint64) (LazyValue, bool) {
	return h.getShard(fileNum, key).get(fileNum, key)
}

func (h *hashMap) Delete(fileNum, key uint64) bool {
	return h.getShard(fileNum, key).delete(fileNum, key)
}

func (h *hashMap) Close() {
	for _, s := range h.shards {
		s.close()
	}
}

func (h *hashMap) SetCapacity(capacity int64) {
	perShardCap := capacity / int64(h.shardNum)
	for _, s := range h.shards {
		s.setCapacity(perShardCap)
	}
}

func (h *hashMap) getShard(fileNum, key uint64) *shard {
	// Implement Fibonacci hashing to evenly distribute keys across shards
	// and avoid clustering of sequential keys.

	const m = 11400714819323198485

	k := fileNum * m
	k ^= key * m
	k >>= 32

	// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return h.shards[int(k*uint64(h.shardNum)>>32)]
}

func NewMap(opts ...CacheOpt) IBlockCache {
	c := &hashMap{
		shardNum:  defaultShardNum,
		maxSize:   defaultCacheSize,
		cacheType: defaultCacheType,
	}

	for _, opt := range opts {
		opt(c)
	}

	minShardSize := 4 * MiB
	if c.shardNum > defaultShardNum && c.maxSize/int64(c.shardNum) < minShardSize {
		c.shardNum = defaultShardNum
	}
	c.shards = make([]*shard, c.shardNum)
	for i := range c.shardNum {
		shardMaxSize := (c.maxSize + int64(c.shardNum-1)) / int64(c.shardNum) // round up
		c.shards[i] = newShard(shardMaxSize, c.cacheType)
	}

	return c
}

var _ IBlockCache = (*hashMap)(nil)
