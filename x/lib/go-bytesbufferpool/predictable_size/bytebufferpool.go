package predictable_size

import (
	"bytes"
	"math/bits"
	"sync"
)

const (
	maximumPoolCnt = 24
)

type PredictablePool struct {
	// pools contains pools for slices of byte of various capacities.
	//
	//	pools[0] is for capacities from 0 upto 256
	//	pools[1] is for capacities from 257 upto 512
	//	pools[2] is for capacities from 513 upto 1024
	//	...
	//	pools[n] is for capacities from 2^(n+7)+1 to 2^(n+8)
	pools [maximumPoolCnt]sync.Pool
}

func NewPredictablePool() *PredictablePool {
	return &PredictablePool{
		pools: [maximumPoolCnt]sync.Pool{},
	}
}

// Get the buffer from the pool. Note that the buffer always has 0 size
func (p *PredictablePool) Get(dataLen int) []byte {
	id, poolCap := getPoolIDAndCapacity(dataLen)
	for range 1 {
		if id >= len(p.pools) {
			break
		}
		if b := p.pools[id].Get(); b != nil {
			return b.(*bytes.Buffer).Bytes()
		}
		id++
	}

	// if the pool is empty, then allocate new poolCap bytes
	return make([]byte, 0, poolCap)
}

func (p *PredictablePool) Put(buf []byte) {
	capacity := cap(buf)
	id, poolCap := getPoolIDAndCapacity(capacity)
	if capacity > poolCap {
		// if the cap of the buf is greater than the maximum threshold, 2^32 ~ 8MB
		// there are no point to cache it by putting back to the pool
		return
	}

	// reset the buffer and remains the capacity, and put into the pool
	buf = buf[:0]
	p.pools[id].Put(bytes.NewBuffer(buf))
}

// getPoolIDAndCapacity predict the poolId from given data size
// and return the pool maximum capacity
func getPoolIDAndCapacity(size int) (int, int) {
	size--
	size = max(size, 0)
	size >>= 8
	id := bits.Len(uint(size))
	id = min(id, maximumPoolCnt-1)
	return id, 1 << (id + 8)
}
