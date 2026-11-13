package go_adaptive_rate_limiter

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultRefillPerTune   = 100
	lowWatermarkPct        = 50
	highWatermarkPct       = 80
	defaultAdjustFactorPct = 5
	defaultMaxLimit        = 10
	defaultMinLimit        = 1
)

type request struct {
	requestedTokens float64
	ready           chan struct{}
}

func newRequest(tokens float64) *request {
	return &request{
		requestedTokens: tokens,
		ready:           make(chan struct{}, 1),
	}
}

func (r *request) signal() {
	select {
	case r.ready <- struct{}{}:
	default:
	}
}

func (r *request) wait(mu *sync.Mutex) {
	// release the lock so other threads can continue the work
	// during this one is still waiting
	mu.Unlock()
	<-r.ready
	mu.Lock()
}

type AdaptiveRateLimiter struct {
	iBucket

	mu sync.Mutex
	// limitPerSec controls the operation rate per second
	limitPerSec atomic.Int64
	queue       *list.List

	// used for auto-tuning
	minLimit    int64
	maxLimit    int64
	lastTunedAt time.Time
	numDrained  int64

	isWaitingUntilNextRefill bool

	// refillPerTune auto-tuning happens every refillPerTune-th of the refill period
	// to avoid frequent adjustment. Higher value means the tuning adjustment will
	// be triggered less frequent
	refillPerTune int64
	// adjustFactorPct used to adjust the limitPerSec value by its adjustFactorPct%
	adjustFactorPct int
}

func NewAdaptiveRateLimiter(opts ...Option) *AdaptiveRateLimiter {
	rl := &AdaptiveRateLimiter{
		lastTunedAt: time.Now(),
		queue:       list.New(),
	}
	rl.ensureDefaultConfig()

	for _, o := range opts {
		o(rl)
	}

	go func() {
		for {
			rl.tune()
		}
	}()

	return rl
}

func (arl *AdaptiveRateLimiter) ensureDefaultConfig() {
	arl.maxLimit = defaultMaxLimit
	arl.minLimit = defaultMinLimit
	arl.refillPerTune = defaultRefillPerTune
	arl.adjustFactorPct = defaultAdjustFactorPct
	arl.iBucket = newBucket(0)
	arl.SetLimitPerSec(arl.minLimit)
}

func (arl *AdaptiveRateLimiter) SetLimitPerSec(limitPerSec int64) {
	arl.limitPerSec.Store(limitPerSec)
	arl.setRefillCountPerPeriodUs(limitPerSec)
}

func (arl *AdaptiveRateLimiter) Wait(ctx context.Context) error {
	return arl.WaitN(ctx, 1)
}

func (arl *AdaptiveRateLimiter) WaitN(ctx context.Context, n int) error {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	requestedTokens := float64(n)

	if arl.getAvailableTokens() > 0 {
		tokenThrough := min(arl.getAvailableTokens(), requestedTokens)
		arl.setAvailableTokens(arl.getAvailableTokens() - tokenThrough)
		requestedTokens -= tokenThrough
	}

	if requestedTokens == 0 {
		return nil
	}

	r := newRequest(requestedTokens)
	arl.queue.PushBack(r)

	maxRefillTokenPerPeriodUs := arl.estimateRefillCountPerPeriodUs(arl.maxLimit)
	if deadline, ok := ctx.Deadline(); ok {
		if float64(deadline.UnixMicro())*maxRefillTokenPerPeriodUs/float64(arl.getRefillPeriodUs()) < float64(n) {
			return fmt.Errorf("rate: Wait(n=%d) would exceed context deadline", n)
		}
	}

	for r.requestedTokens > 0 {
		timeUntilNextRefillUs := arl.getNextRefillAt() - time.Now().UnixMicro()
		if timeUntilNextRefillUs > 0 {
			if arl.isWaitingUntilNextRefill {
				r.wait(&arl.mu)
			} else {
				waitUntilUs := time.Now().UnixMicro() + timeUntilNextRefillUs
				arl.numDrained++
				arl.isWaitingUntilNextRefill = true
				arl.doWait(waitUntilUs)
				arl.isWaitingUntilNextRefill = false
			}
		} else {
			arl.refillAndGrants()
		}

		if r.requestedTokens == 0 {
			if arl.queue.Len() > 0 {
				frontReq := arl.queue.Front().Value.(*request)
				frontReq.signal()
			}
		}
	}

	return nil
}

func (arl *AdaptiveRateLimiter) refillAndGrants() {
	nextRefillAt := time.Now().UnixMicro() + arl.getRefillPeriodUs()
	arl.setNextRefillAt(nextRefillAt)
	availableTokens := arl.getAvailableTokens() + arl.getRefillCountPerPeriodUs()

	for arl.queue.Len() > 0 {
		nextRequest := arl.queue.Front().Value.(*request)
		if availableTokens < nextRequest.requestedTokens {
			nextRequest.requestedTokens -= availableTokens
			availableTokens = 0
			break
		}

		availableTokens -= nextRequest.requestedTokens
		nextRequest.requestedTokens = 0
		arl.queue.Remove(arl.queue.Front())
		nextRequest.signal()
	}

	arl.setAvailableTokens(availableTokens)
}

func (arl *AdaptiveRateLimiter) doWait(waitUntilUs int64) {
	nowUs := time.Now().UnixMicro()
	if waitUntilUs <= nowUs {
		return
	}

	delay := waitUntilUs - nowUs
	arl.mu.Unlock()
	time.Sleep(time.Duration(delay) * time.Microsecond)
	arl.mu.Lock()
}

func (arl *AdaptiveRateLimiter) tune() {
	if time.Since(arl.lastTunedAt).Microseconds() < arl.refillPerTune*arl.getRefillPeriodUs() {
		return
	}

	prevTunedTime := arl.lastTunedAt
	arl.lastTunedAt = time.Now()

	diff := arl.lastTunedAt.Sub(prevTunedTime).Microseconds()
	refillPeriodUs := arl.getRefillPeriodUs()
	elapsedInterval := float64(diff+refillPeriodUs-1) / float64(refillPeriodUs)

	drainedPct := 100.0 * float64(arl.numDrained) / elapsedInterval

	prevLimit := arl.limitPerSec.Load()
	var newLimit int64

	switch {
	case drainedPct < lowWatermarkPct:
		sanitizedPrevLimit := min(prevLimit, math.MaxInt64/100) // prevent from overflow
		newLimit = max(arl.minLimit, sanitizedPrevLimit*100/int64(100+arl.adjustFactorPct))
	case drainedPct > highWatermarkPct:
		sanitizedPrevLimit := min(prevLimit, math.MaxInt64/int64(100+arl.adjustFactorPct)) // prevent from overflow
		newLimit = min(arl.maxLimit, sanitizedPrevLimit*int64(100+arl.adjustFactorPct)/100)
	default:
		newLimit = prevLimit
	}

	if newLimit != prevLimit {
		arl.SetLimitPerSec(newLimit)
	}

	atomic.StoreInt64(&arl.numDrained, 0)
}
