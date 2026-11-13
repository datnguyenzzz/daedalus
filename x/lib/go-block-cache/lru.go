package go_block_cache

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

type lru struct {
	inUse    int64
	capacity int64

	mu sync.Mutex

	// recent dummy node.
	//   dummy recent <--> 1st most recent  <--> 2nd most recent
	//   ^                                                     ^
	//   |                                                     |
	//   v                                                     v
	//   least recent <-->       ...       <--> K-th most recent
	recent *log
}

func newLRU(maxSize int64) *lru {
	dummy := new(log)
	dummy.next = dummy
	dummy.prev = dummy
	return &lru{
		capacity: maxSize,
		recent:   dummy,
	}
}

func (l *lru) SetCapacity(capacity int64) {
	l.mu.Lock()
	l.capacity = capacity
	evicted := l.balance()
	l.mu.Unlock()

	for _, n := range evicted {
		n.s.mu.Lock()
		n.s.evict(n)
		n.s.mu.Unlock()
	}
}

func (l *lru) Promote(node *kv, diffSize int64, o op) bool {
	l.mu.Lock()
	if node.log == nil {
		// the key/value pair is updated for the first time
		if node.size > l.capacity {
			zap.L().Error("node is bigger than eviction value")
			return false
		}

		log := &log{n: node}
		l.recent.insert(log)
		node.log = unsafe.Pointer(log)
		atomic.AddInt64(&l.inUse, node.size)
	} else {
		log := (*log)(node.log)
		log.remove()
		l.recent.insert(log)
		atomic.AddInt64(&l.inUse, diffSize)
	}
	evicted := l.balance()
	l.mu.Unlock()

	for _, n := range evicted {
		n.s.mu.Lock()
		n.s.evict(n)
		n.s.mu.Unlock()
	}

	return true
}

func (l *lru) Evict(node *kv) {
	l.mu.Lock()
	defer l.mu.Unlock()

	currLog := (*log)(node.log)
	if currLog == nil {
		return
	}
	l.removeLRULog(currLog)
}

// balance evict nodes to balance the maxSize.
//
//	Caller must ensure the lru is locked
func (l *lru) balance() (evicted []*kv) {
	for l.inUse > l.capacity {
		leastUpdate := l.recent.prev
		if leastUpdate == nil {
			panic("lru recent pointer is nil")
		}
		l.removeLRULog(leastUpdate)
		evicted = append(evicted, leastUpdate.n)
	}

	return evicted
}

func (l *lru) removeLRULog(lruLog *log) {
	lruLog.remove()
	lruLog.n.log = nil
	atomic.AddInt64(&l.inUse, -int64(computeSize(lruLog.n.value)))
}

func (l *lru) GetInUsed() int64 {
	return atomic.LoadInt64(&l.inUse)
}

var _ ICacher = (*lru)(nil)
