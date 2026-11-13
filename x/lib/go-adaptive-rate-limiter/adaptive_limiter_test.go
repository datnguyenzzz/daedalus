package go_adaptive_rate_limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewAdaptiveRateLimiter(t *testing.T) {
	type param struct {
		desc                string
		opts                []Option
		expectedMinLimit    int64
		expectedMaxLimit    int64
		expectedAdjustPct   int
		expectedLimitPerSec int64
	}

	testList := []param{
		{
			desc:                "default configuration",
			opts:                nil,
			expectedMinLimit:    defaultMinLimit,
			expectedMaxLimit:    defaultMaxLimit,
			expectedAdjustPct:   defaultAdjustFactorPct,
			expectedLimitPerSec: defaultMinLimit,
		},
		{
			desc: "custom limits",
			opts: []Option{
				WithLimit(5, 100),
			},
			expectedMinLimit:    5,
			expectedMaxLimit:    100,
			expectedAdjustPct:   defaultAdjustFactorPct,
			expectedLimitPerSec: 5,
		},
		{
			desc: "custom adjust factor",
			opts: []Option{
				WithAdjustFactorPct(10),
			},
			expectedMinLimit:    defaultMinLimit,
			expectedMaxLimit:    defaultMaxLimit,
			expectedAdjustPct:   10,
			expectedLimitPerSec: defaultMinLimit,
		},
		{
			desc: "all custom options",
			opts: []Option{
				WithLimit(20, 200),
				WithAdjustFactorPct(15),
				WithRefillPeriodUs(50_000),
			},
			expectedMinLimit:    20,
			expectedMaxLimit:    200,
			expectedAdjustPct:   15,
			expectedLimitPerSec: 20,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(tc.opts...)

			assert.Equal(t, tc.expectedMinLimit, limiter.minLimit, "minLimit should match")
			assert.Equal(t, tc.expectedMaxLimit, limiter.maxLimit, "maxLimit should match")
			assert.Equal(t, tc.expectedAdjustPct, limiter.adjustFactorPct, "adjustFactorPct should match")
			assert.Equal(t, tc.expectedLimitPerSec, limiter.limitPerSec.Load(), "limitPerSec should match")
			assert.Equal(t, int64(defaultRefillPerTune), limiter.refillPerTune, "refillPerTune should be default")
			assert.NotNil(t, limiter.queue, "queue should be initialized")
		})
	}
}

func TestAdaptiveRateLimiter_SetLimitPerSec(t *testing.T) {
	type param struct {
		desc           string
		initialLimit   int64
		newLimit       int64
		refillPeriodUs int64
	}

	testList := []param{
		{
			desc:           "increase limit",
			initialLimit:   10,
			newLimit:       50,
			refillPeriodUs: defaultRefillPeriodUs,
		},
		{
			desc:           "decrease limit",
			initialLimit:   100,
			newLimit:       20,
			refillPeriodUs: defaultRefillPeriodUs,
		},
		{
			desc:           "set same limit",
			initialLimit:   50,
			newLimit:       50,
			refillPeriodUs: defaultRefillPeriodUs,
		},
		{
			desc:           "custom refill period",
			initialLimit:   10,
			newLimit:       30,
			refillPeriodUs: 200_000,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithRefillPeriodUs(tc.refillPeriodUs))
			limiter.SetLimitPerSec(tc.initialLimit)

			limiter.SetLimitPerSec(tc.newLimit)

			assert.Equal(t, tc.newLimit, limiter.limitPerSec.Load(), "limitPerSec should be updated")
			expectedRefillCount := float64(tc.newLimit*tc.refillPeriodUs) / float64(time.Second.Microseconds())
			assert.Equal(t, expectedRefillCount, limiter.getRefillCountPerPeriodUs(), "refillCountPerPeriodUs should be updated")
		})
	}
}

func TestAdaptiveRateLimiter_Wait_single_thread(t *testing.T) {
	type param struct {
		desc            string
		limitPerSec     int64
		numWaits        int
		expectError     bool
		maxExpectedTime time.Duration
	}

	testList := []param{
		{
			desc:            "single wait with available tokens",
			limitPerSec:     10,
			numWaits:        1,
			expectError:     false,
			maxExpectedTime: 200 * time.Millisecond,
		},
		{
			desc:            "multiple waits within limit",
			limitPerSec:     20,
			numWaits:        5,
			expectError:     false,
			maxExpectedTime: 500 * time.Millisecond,
		},
		{
			desc:            "sequential waits",
			limitPerSec:     5,
			numWaits:        10,
			expectError:     false,
			maxExpectedTime: 3 * time.Second,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec))
			ctx := context.Background()

			start := time.Now()
			for range tc.numWaits {
				err := limiter.Wait(ctx)
				if tc.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
			elapsed := time.Since(start)

			assert.Less(t, elapsed, tc.maxExpectedTime, "wait should complete within expected time")
		})
	}
}

func TestAdaptiveRateLimiter_WaitN(t *testing.T) {
	type param struct {
		desc            string
		limitPerSec     int64
		tokensRequested int
		expectError     bool
		maxExpectedTime time.Duration
	}

	testList := []param{
		{
			desc:            "request available tokens",
			limitPerSec:     20,
			tokensRequested: 5,
			expectError:     false,
			maxExpectedTime: 500 * time.Millisecond,
		},
		{
			desc:            "request zero tokens",
			limitPerSec:     10,
			tokensRequested: 0,
			expectError:     false,
			maxExpectedTime: 100 * time.Millisecond,
		},
		{
			desc:            "request tokens exceeding immediate capacity",
			limitPerSec:     5,
			tokensRequested: 15,
			expectError:     false,
			maxExpectedTime: 5 * time.Second,
		},
		{
			desc:            "single token request",
			limitPerSec:     50,
			tokensRequested: 1,
			expectError:     false,
			maxExpectedTime: 200 * time.Millisecond,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec))
			ctx := context.Background()

			start := time.Now()
			err := limiter.WaitN(ctx, tc.tokensRequested)
			elapsed := time.Since(start)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Less(t, elapsed, tc.maxExpectedTime, "WaitN should complete within expected time")
		})
	}
}

func TestAdaptiveRateLimiter_Tune(t *testing.T) {
	type param struct {
		desc                string
		initialLimit        int64
		minLimit            int64
		maxLimit            int64
		adjustFactorPct     int
		numDrained          int64
		timeSinceLastTune   time.Duration
		expectLimitIncrease bool
		expectLimitDecrease bool
		expectMinLimit      bool
	}

	testList := []param{
		{
			desc:                "high usage increases limit",
			initialLimit:        50,
			minLimit:            10,
			maxLimit:            100,
			adjustFactorPct:     10,
			numDrained:          950,
			timeSinceLastTune:   20 * time.Second,
			expectLimitIncrease: true,
			expectLimitDecrease: false,
			expectMinLimit:      false,
		},
		{
			desc:                "low usage decreases limit",
			initialLimit:        50,
			minLimit:            10,
			maxLimit:            100,
			adjustFactorPct:     5,
			numDrained:          1,
			timeSinceLastTune:   20 * time.Second,
			expectLimitIncrease: false,
			expectLimitDecrease: true,
			expectMinLimit:      false,
		},
		{
			desc:                "zero usage sets to min limit",
			initialLimit:        50,
			minLimit:            5,
			maxLimit:            100,
			adjustFactorPct:     5,
			numDrained:          0,
			timeSinceLastTune:   20 * time.Second,
			expectLimitIncrease: false,
			expectLimitDecrease: true,
			expectMinLimit:      false,
		},
		{
			desc:                "medium usage maintains limit",
			initialLimit:        50,
			minLimit:            10,
			maxLimit:            100,
			adjustFactorPct:     5,
			numDrained:          150,
			timeSinceLastTune:   20 * time.Second,
			expectLimitIncrease: false,
			expectLimitDecrease: false,
			expectMinLimit:      false,
		},
		{
			desc:                "respects max limit",
			initialLimit:        98,
			minLimit:            10,
			maxLimit:            100,
			adjustFactorPct:     10,
			numDrained:          980,
			timeSinceLastTune:   20 * time.Second,
			expectLimitIncrease: true,
			expectLimitDecrease: false,
			expectMinLimit:      false,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(
				WithLimit(tc.minLimit, tc.maxLimit),
				WithAdjustFactorPct(tc.adjustFactorPct),
			)
			limiter.SetLimitPerSec(tc.initialLimit)
			limiter.lastTunedAt = time.Now().Add(-tc.timeSinceLastTune)
			atomic.StoreInt64(&limiter.numDrained, tc.numDrained)

			initialLimit := limiter.limitPerSec.Load()
			limiter.tune()
			newLimit := limiter.limitPerSec.Load()

			if tc.expectLimitIncrease {
				assert.Greater(t, newLimit, initialLimit, "limit should increase")
				assert.LessOrEqual(t, newLimit, tc.maxLimit, "limit should not exceed max")
			} else if tc.expectLimitDecrease {
				assert.Less(t, newLimit, initialLimit, "limit should decrease")
				assert.GreaterOrEqual(t, newLimit, tc.minLimit, "limit should not go below min")
			} else if tc.expectMinLimit {
				assert.Equal(t, tc.minLimit, newLimit, "limit should be set to min")
			} else if tc.initialLimit >= tc.maxLimit {
				assert.LessOrEqual(t, newLimit, tc.maxLimit, "limit should not exceed max")
			} else {
				assert.Equal(t, initialLimit, newLimit, "limit should remain unchanged")
			}

			assert.Equal(t, int64(0), atomic.LoadInt64(&limiter.numDrained), "numDrained should be reset")
		})
	}
}

func TestAdaptiveRateLimiter_ConcurrentWait(t *testing.T) {
	type param struct {
		desc                   string
		limitPerSec            int64
		concurrency            int
		operationsPerGoroutine int
		maxExpectedTime        time.Duration
	}

	testList := []param{
		{
			desc:                   "low concurrency",
			limitPerSec:            50,
			concurrency:            10,
			operationsPerGoroutine: 5,
			maxExpectedTime:        3 * time.Second,
		},
		{
			desc:                   "medium concurrency",
			limitPerSec:            100,
			concurrency:            50,
			operationsPerGoroutine: 10,
			maxExpectedTime:        10 * time.Second,
		},
		{
			desc:                   "high concurrency",
			limitPerSec:            200,
			concurrency:            100,
			operationsPerGoroutine: 5,
			maxExpectedTime:        5 * time.Second,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec*2))
			var wg sync.WaitGroup
			var successCount atomic.Int64

			start := time.Now()
			for range tc.concurrency {
				wg.Go(func() {
					for range tc.operationsPerGoroutine {
						err := limiter.Wait(context.Background())
						if err == nil {
							successCount.Add(1)
						}
					}
				})
			}

			wg.Wait()
			elapsed := time.Since(start)

			expectedOps := int64(tc.concurrency * tc.operationsPerGoroutine)
			assert.Equal(t, expectedOps, successCount.Load(), "all operations should succeed")
			assert.Less(t, elapsed, tc.maxExpectedTime, "concurrent waits should complete within expected time")
		})
	}
}

func TestAdaptiveRateLimiter_RateLimiting(t *testing.T) {
	type param struct {
		desc                string
		qps                 int64
		operations          int
		minExpectedDuration time.Duration
		maxExpectedDuration time.Duration
	}

	testList := []param{
		{
			desc:                "10 QPS rate limit",
			qps:                 10,
			operations:          20,
			minExpectedDuration: 1500 * time.Millisecond,
			maxExpectedDuration: 2500 * time.Millisecond,
		},
		{
			desc:                "20 QPS rate limit",
			qps:                 20,
			operations:          30,
			minExpectedDuration: 1000 * time.Millisecond,
			maxExpectedDuration: 2000 * time.Millisecond,
		},
		{
			desc:                "50 QPS rate limit",
			qps:                 50,
			operations:          100,
			minExpectedDuration: 1500 * time.Millisecond,
			maxExpectedDuration: 2500 * time.Millisecond,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.qps, tc.qps))

			start := time.Now()
			for range tc.operations {
				err := limiter.Wait(context.Background())
				assert.NoError(t, err)
			}
			elapsed := time.Since(start)

			assert.GreaterOrEqual(t, elapsed, tc.minExpectedDuration, "should enforce minimum rate")
			assert.Less(t, elapsed, tc.maxExpectedDuration, "should not take too long")
		})
	}
}

func TestAdaptiveRateLimiter_RefillAndGrants(t *testing.T) {
	type param struct {
		desc            string
		limitPerSec     int64
		requests        []float64
		availableTokens float64
		expectedQueue   int
	}

	testList := []param{
		{
			desc:            "single request fulfilled",
			limitPerSec:     10,
			requests:        []float64{5},
			availableTokens: 10,
			expectedQueue:   0,
		},
		{
			desc:            "multiple requests fulfilled",
			limitPerSec:     20,
			requests:        []float64{3, 5, 2},
			availableTokens: 20,
			expectedQueue:   0,
		},
		{
			desc:            "partial fulfillment",
			limitPerSec:     5,
			requests:        []float64{10},
			availableTokens: 5,
			expectedQueue:   1,
		},
		{
			desc:            "insufficient tokens for all requests",
			limitPerSec:     10,
			requests:        []float64{5, 8, 3},
			availableTokens: 10,
			expectedQueue:   2,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec))

			for _, tokens := range tc.requests {
				req := newRequest(tokens)
				limiter.queue.PushBack(req)
			}

			limiter.setAvailableTokens(tc.availableTokens)
			limiter.refillAndGrants()

			assert.Equal(t, tc.expectedQueue, limiter.queue.Len(), "queue length should match")
		})
	}
}

func TestAdaptiveRateLimiter_EdgeCases(t *testing.T) {
	type param struct {
		desc        string
		limitPerSec int64
		operation   func(limiter *AdaptiveRateLimiter) error
		expectError bool
	}

	testList := []param{
		{
			desc:        "zero token request",
			limitPerSec: 10,
			operation: func(limiter *AdaptiveRateLimiter) error {
				return limiter.WaitN(context.Background(), 0)
			},
			expectError: false,
		},
		{
			desc:        "large token request",
			limitPerSec: 10,
			operation: func(limiter *AdaptiveRateLimiter) error {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				return limiter.WaitN(ctx, 50)
			},
			expectError: false,
		},
		{
			desc:        "cancelled context",
			limitPerSec: 1,
			operation: func(limiter *AdaptiveRateLimiter) error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return limiter.Wait(ctx)
			},
			expectError: false,
		},
		{
			desc:        "rapid successive calls",
			limitPerSec: 100,
			operation: func(limiter *AdaptiveRateLimiter) error {
				for range 10 {
					if err := limiter.Wait(context.Background()); err != nil {
						return err
					}
				}
				return nil
			},
			expectError: false,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec*5))
			err := tc.operation(limiter)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAdaptiveRateLimiter_AdaptiveBehavior(t *testing.T) {
	type param struct {
		desc        string
		minLimit    int64
		maxLimit    int64
		adjustPct   int
		operations  int
		expectAdapt bool
	}

	testList := []param{
		{
			desc:        "adapts under sustained load",
			minLimit:    10,
			maxLimit:    100,
			adjustPct:   10,
			operations:  100,
			expectAdapt: true,
		},
		{
			desc:        "adapts with high adjust factor",
			minLimit:    5,
			maxLimit:    50,
			adjustPct:   20,
			operations:  100,
			expectAdapt: true,
		},
		{
			desc:        "limited range adaptation",
			minLimit:    40,
			maxLimit:    50,
			adjustPct:   5,
			operations:  100,
			expectAdapt: false,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(
				WithLimit(tc.minLimit, tc.maxLimit),
				WithAdjustFactorPct(tc.adjustPct),
			)

			initialLimit := limiter.limitPerSec.Load()

			ctx := context.Background()
			for range tc.operations {
				limiter.Wait(ctx)
			}

			finalLimit := limiter.limitPerSec.Load()

			if tc.expectAdapt {
				assert.NotEqual(t, initialLimit, finalLimit, "limit should adapt")
			}
			assert.GreaterOrEqual(t, finalLimit, tc.minLimit, "should not go below min")
			assert.LessOrEqual(t, finalLimit, tc.maxLimit, "should not exceed max")
		})
	}
}

func TestAdaptiveRateLimiter_QueueManagement(t *testing.T) {
	type param struct {
		desc              string
		limitPerSec       int64
		concurrentWaits   int
		expectedQueueGrow bool
	}

	testList := []param{
		{
			desc:              "queue grows under load",
			limitPerSec:       5,
			concurrentWaits:   20,
			expectedQueueGrow: true,
		},
		{
			desc:              "queue empties after waits complete",
			limitPerSec:       50,
			concurrentWaits:   10,
			expectedQueueGrow: false,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			limiter := NewAdaptiveRateLimiter(WithLimit(tc.limitPerSec, tc.limitPerSec*2))

			assert.Equal(t, 0, limiter.queue.Len(), "queue should start empty")

			var wg sync.WaitGroup
			for range tc.concurrentWaits {
				wg.Go(func() {
					limiter.Wait(context.Background())
				})
			}

			time.Sleep(50 * time.Millisecond)

			limiter.mu.Lock()
			queueLen := limiter.queue.Len()
			limiter.mu.Unlock()

			if tc.expectedQueueGrow {
				assert.Greater(t, queueLen, 0, "queue should have items")
			}

			wg.Wait()

			limiter.mu.Lock()
			finalQueueLen := limiter.queue.Len()
			limiter.mu.Unlock()

			assert.Equal(t, 0, finalQueueLen, "queue should be empty after completion")
		})
	}
}
