# go-adaptive-rate-limiter

A high-performance adaptive rate limiter for Go that automatically adjusts its rate limit based on real-time usage patterns. Built on a token bucket algorithm with microsecond precision, it dynamically scales between configurable minimum and maximum limits to optimize throughput while preventing resource exhaustion.

## Features

- **Adaptive Rate Limiting**: Automatically adjusts rate limits based on actual usage patterns
- **Token Bucket Implementation**: Fine-grained control with microsecond-level precision
- **Concurrent Safe**: Thread-safe operations with minimal lock contention
- **Configurable Parameters**: Customize min/max limits, adjustment factors, and refill periods
- **Context Support**: Respects context cancellation and deadlines
- **Zero Allocation Fast Path**: Optimized for high-throughput scenarios

## Installation

```bash
go get github.com/datnguyenzzz/nogodb/lib/go-adaptive-rate-limiter
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    rl "github.com/datnguyenzzz/nogodb/lib/go-adaptive-rate-limiter"
)

func main() {
    // Create a rate limiter that adapts between 10-100 requests per second
    limiter := rl.NewAdaptiveRateLimiter(
        rl.WithLimit(10, 100),
    )

    // Wait for a single token
    if err := limiter.Wait(context.Background()); err != nil {
        fmt.Println("Rate limit error:", err)
        return
    }

    // Your rate-limited operation here
    fmt.Println("Request allowed")
}
```

## Usage

### Basic Usage

```go
// Create with default settings (min: 1, max: 10 QPS)
limiter := rl.NewAdaptiveRateLimiter()

// Wait for permission to proceed
err := limiter.Wait(context.Background())
```

### Request Multiple Tokens

```go
// Request 5 tokens at once
err := limiter.WaitN(context.Background(), 5)
```

### Custom Configuration

```go
limiter := rl.NewAdaptiveRateLimiter(
    // Set minimum and maximum QPS
    rl.WithLimit(50, 500),

    // Set adjustment factor percentage (default: 5%)
    // Higher values = faster adaptation
    rl.WithAdjustFactorPct(10),

    // Set refill period in microseconds (default: 100ms)
    rl.WithRefillPeriodUs(50_000),
)
```

### With Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := limiter.Wait(ctx); err != nil {
    // Handle timeout or cancellation
    return err
}
```

## How It Works

### Adaptive Algorithm

The rate limiter continuously monitors usage and adjusts the rate limit based on demand:

1. **High Usage (≥80% drained)**: Increases limit by `adjustFactorPct`
2. **Low Usage (≤50% drained)**: Decreases limit by `adjustFactorPct`
3. **No Usage**: Drops to minimum limit
4. **Medium Usage (50-80%)**: Maintains current limit

The limiter samples usage every `refillPerTune` refill periods (default: 100 periods) to avoid over-adjustment.

### Token Bucket Model

- Tokens are refilled periodically based on `refillPeriodUs`
- Each `Wait()` consumes one token
- `WaitN(n)` consumes n tokens
- Requests queue when tokens are unavailable
- Automatic queue management prevents starvation

## Configuration Options

### WithLimit(minQps, maxQps int64)

Sets the minimum and maximum queries per second.

- `minQps`: Lower bound for adaptive adjustment
- `maxQps`: Upper bound for adaptive adjustment

**Default**: min=1, max=10

```go
rl.WithLimit(100, 1000) // Adapts between 100-1000 QPS
```

### WithAdjustFactorPct(pct int)

Sets the percentage by which the rate limit adjusts.

- Higher values = faster adaptation but more volatility
- Lower values = smoother adaptation but slower response

**Default**: 5%

```go
rl.WithAdjustFactorPct(15) // Adjusts by 15% each time
```

### WithRefillPeriodUs(microseconds int64)

Sets how frequently tokens are refilled.

- Smaller values = finer granularity but more CPU overhead
- Larger values = lower overhead but potential burst behavior

**Default**: 100,000 microseconds (100ms)

```go
rl.WithRefillPeriodUs(50_000) // Refill every 50ms
```

## Performance

Benchmarks show the adaptive rate limiter can handle various traffic patterns efficiently:

- **Square Wave**: Handles sudden traffic spikes by quickly scaling up
- **Sine Wave**: Smoothly adapts to gradual traffic changes
- **Cosine Wave**: Efficiently tracks phase-shifted patterns
- **Rectangular Wave**: Manages alternating high/low traffic periods

See `bench/` directory for detailed benchmark results and visualizations.

## Thread Safety

All methods are safe for concurrent use. The implementation uses:

- Atomic operations for frequently-read values
- Minimal lock contention through fine-grained locking
- Lock-free fast path for available tokens

## Examples

### Batch Operations

```go
// Request multiple tokens for batch operations
batchSize := 10
if err := limiter.WaitN(ctx, batchSize); err != nil {
    return err
}

// Process batch of items
processBatch(items[:batchSize])
```

## Testing

Run the test suite:

```bash
go test -v
```

Run benchmarks:

```bash
cd bench
go test -bench=. -benchtime=10s
```