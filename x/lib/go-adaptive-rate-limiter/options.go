package go_adaptive_rate_limiter

type Option func(arl *AdaptiveRateLimiter)

func WithLimit(minQps, maxQps int64) Option {
	return func(rl *AdaptiveRateLimiter) {
		rl.maxLimit = maxQps
		rl.minLimit = minQps
		rl.SetLimitPerSec(minQps)
	}
}

func WithAdjustFactorPct(adjustFactorPct int) Option {
	return func(rl *AdaptiveRateLimiter) {
		rl.adjustFactorPct = adjustFactorPct
	}
}

func WithRefillPeriodUs(refillPeriodUs int64) Option {
	return func(rl *AdaptiveRateLimiter) {
		rl.setRefillPeriodUs(refillPeriodUs)
	}
}
