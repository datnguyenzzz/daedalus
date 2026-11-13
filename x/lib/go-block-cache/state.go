package go_block_cache

import (
	"sort"
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"
)

const (
	overflowThreshold     = 1 << 5
	overflowGrowThreshold = 1 << 7
)

// state list of bucket, represent the current state of the hashmap
type state struct {
	buckets    []bucket
	bucketMark uint32

	prevState unsafe.Pointer // point to the previous state

	// resizing True if any bucket is changing its size
	resizing int32

	// overflow number of buckets that have size larger than overflowThreshold
	overflow        int32
	growThreshold   int64
	shrinkThreshold int64
}

func (s *state) lazyLoadBucket(id uint32) *bucket {
	bucket := &s.buckets[id]

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if bucket.state >= initialized {
		return bucket
	}

	// fmt.Printf("init bucket %v\n", unsafe.Pointer(bucket))

	prevState := (*state)(atomic.LoadPointer(&s.prevState))
	if prevState == nil {
		msg := "prev state is nil when init a fresh bucket"
		zap.L().Error(msg)
		panic(msg)
	}

	if s.bucketMark > prevState.bucketMark {
		// grow
		nodes := prevState.lazyLoadBucket(id & prevState.bucketMark).Freeze()
		for _, node := range nodes {
			if node.hash&s.bucketMark == id {
				bucket.nodes = append(bucket.nodes, node)
			}
		}
	} else {
		// shrink
		nodes0 := prevState.lazyLoadBucket(id).Freeze()
		nodes1 := prevState.lazyLoadBucket(id + uint32(len(s.buckets))).Freeze()

		bucket.nodes = make([]*kv, 0, len(nodes0)+len(nodes1))
		bucket.nodes = append(bucket.nodes, nodes0...)
		bucket.nodes = append(bucket.nodes, nodes1...)
		sort.Slice(bucket.nodes, func(i, j int) bool {
			return bucket.nodes[i].key < bucket.nodes[j].key ||
				(bucket.nodes[i].key == bucket.nodes[j].key && bucket.nodes[i].fileNum < bucket.nodes[j].fileNum)
		})
	}

	bucket.state = initialized
	return bucket
}

func (s *state) initBuckets() {
	for i := range s.buckets {
		s.lazyLoadBucket(uint32(i))
	}

	atomic.StorePointer(&s.prevState, nil)
}
