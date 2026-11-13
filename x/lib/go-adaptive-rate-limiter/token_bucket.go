package go_adaptive_rate_limiter

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRefillPeriodUs = int64(100_000)
)

type bucket struct {
	mu sync.Mutex

	// tokens available tokens in the bucket
	tokens float64
	// refillPeriodUs controls how often tokens are refilled.
	// Larger value can lead to burst writes while smaller value introduces more CPU overhead
	refillPeriodUs int64
	// refillCountPerPeriodUs controls number of token for each round of re-filling tokens
	refillCountPerPeriodUs float64
	nextRefillAtUs         int64
}

type iBucket interface {
	setAvailableTokens(tokens float64)
	getAvailableTokens() float64

	getNextRefillAt() int64
	setNextRefillAt(nextRefillAt int64)

	getRefillPeriodUs() int64
	setRefillPeriodUs(refillPeriodUs int64)

	getRefillCountPerPeriodUs() float64
	setRefillCountPerPeriodUs(operationsPerSec int64)
	estimateRefillCountPerPeriodUs(operationsPerSec int64) float64
}

func newBucket(refillPeriodUs int64) *bucket {
	b := &bucket{}
	atomic.StoreInt64(&b.nextRefillAtUs, time.Now().UnixMicro())
	if refillPeriodUs > 0 {
		atomic.StoreInt64(&b.refillPeriodUs, refillPeriodUs)
	} else {
		atomic.StoreInt64(&b.refillPeriodUs, defaultRefillPeriodUs)
	}

	return b
}

func (b *bucket) setNextRefillAt(nextRefillAt int64) {
	atomic.StoreInt64(&b.nextRefillAtUs, nextRefillAt)
}

// getNextRefillAt get next refill at time in Micro Second
func (b *bucket) getNextRefillAt() int64 {
	return atomic.LoadInt64(&b.nextRefillAtUs)
}

func (b *bucket) getAvailableTokens() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens
}

func (b *bucket) setAvailableTokens(tokens float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = tokens
}

func (b *bucket) getRefillPeriodUs() int64 {
	return atomic.LoadInt64(&b.refillPeriodUs)
}

func (b *bucket) setRefillPeriodUs(refillPeriodUs int64) {
	atomic.StoreInt64(&b.refillPeriodUs, refillPeriodUs)
}

func (b *bucket) getRefillCountPerPeriodUs() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.refillCountPerPeriodUs
}

func (b *bucket) setRefillCountPerPeriodUs(operationsPerSec int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if math.MaxFloat64/float64(operationsPerSec) < float64(b.refillPeriodUs) {
		b.refillCountPerPeriodUs = math.MaxFloat64 / float64(time.Second.Microseconds())
	} else {
		b.refillCountPerPeriodUs = float64(operationsPerSec*b.refillPeriodUs) / float64(time.Second.Microseconds())
	}
}

func (b *bucket) estimateRefillCountPerPeriodUs(operationsPerSec int64) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	if math.MaxFloat64/float64(operationsPerSec) < float64(b.refillPeriodUs) {
		return math.MaxFloat64 / float64(time.Second.Microseconds())
	}

	return float64(operationsPerSec*b.refillPeriodUs) / float64(time.Second.Microseconds())
}
