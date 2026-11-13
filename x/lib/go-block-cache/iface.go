package go_block_cache

type LazyValue interface {
	Load() Value
	Release()
}

type IBlockCache interface {
	Get(fileNum, key uint64) (LazyValue, bool)
	Set(fileNum, key uint64, value Value) bool
	Delete(fileNum, key uint64) bool
	Close()
	SetCapacity(capacity int64)

	// utils

	GetStats() Stats
	GetInUsed() int64
}

// ICacher is the interface for the cache replacement policy, which is used to promote or evict nodes in the cache.
type ICacher interface {
	// GetInUsed returns the current in-use size of the cache,
	GetInUsed() int64
	// Promote promotes the given node in the cache
	// diffSize is the size difference between the new value and the old value of the node
	Promote(node *kv, diffSize int64, o op) bool
	Evict(node *kv)
	SetCapacity(capacity int64)
}
