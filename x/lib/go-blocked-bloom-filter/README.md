# Blocked Bloom Filter with Bit Patterns

A Go implementation of cache-efficient Blocked Bloom Filters that uses bit patterns to improve performance.

## Overview

This package provides a memory and CPU-efficient implementation of Blocked Bloom Filters with Bit Patterns, optimized for:

- Cache locality (blocks aligned to CPU cache lines)
- Reduced hash function calculations
- Configurable false positive rates
- High performance membership testing
- **Cache-Optimized**: Aligns blocks to 64-byte CPU cache lines for improved memory access
- **Efficient Hashing**: Uses only 2 hash functions instead of k separate hash functions
- **Bit Pattern Design**: Pre-computed bit patterns for fast setting/checking of bits
- **Configurable Precision**: Adjustable bits-per-key ratio for controlling false positive rates
- **Simple API**: Easy-to-use interface for adding elements and querying membership

## Usage

```go
import "github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter"

// Create a new Bloom filter
bf := go_blocked_bloom_filter.NewBloomFilter()

// Get a writer to populate the filter
writer := bf.NewWriter()

// Add elements
writer.Add([]byte("hello"))
writer.Add([]byte("world"))

// Build the filter
var filter []byte
writer.Build(&filter)

// Query membership
isPresent := bf.MayContain(filter, []byte("hello")) // true
isAbsent := bf.MayContain(filter, []byte("goodbye")) // false (likely)
```

## How It Works

### Block-Based Structure

The implementation divides the bit array into small blocks that fit into CPU cache lines (64 bytes = 512 bits per block):

```
Bytes form: |  b1, b2, ..., bn |  b1, b2, ..., bn | ...
            | block 1 64 bytes | block 2 64 bytes | ...
Bits form:  |    000...0000    |    000...0000    | ...
            | block 1 512 bits | block 2 512 bits | ...
```

### Two-Level Hashing

1. **Block Selection**: The first hash determines which block to use
2. **Bit Pattern**: The second hash generates a bit pattern (with exactly k bits set to 1)

### Implementation Details

- **Block Size**: Fixed at 64 bytes (512 bits) to match CPU cache line size
- **Default Configuration**: 10 bits per key, optimal for most use cases
- **False Positive Rate**: Approximately 0.01 (1%) with default settings
- **Storage Format**: Data blocks followed by metadata (probe count and block count)


## References

- [Blocked Bloom Filters and Cache Efficient Design](https://save-buffer.github.io/bloom_filter.html)
- [Performance Improvement of Bloom Filters for Networking Applications](https://www.cs.princeton.edu/~chazelle/pubs/FilteringSearch.pdf)
