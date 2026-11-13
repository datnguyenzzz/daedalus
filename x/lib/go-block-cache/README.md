# Go Block Cache

A high-performance, thread-safe block cache implementation built for nogoDB. This library provides a concurrent, 
dynamic-sized hash table with configurable cache replacement policies, optimized for database block caching scenarios.
The hash tables implementation is based on: "Dynamic-Sized Nonblocking Hash Tables", by Yujie Liu, Kunlong Zhang, and 
Michael Spear. ACM Symposium on Principles of Distributed Computing, Jul 2014.

## Key Concepts

Blocks are keyed by a `(fileNum, key)` pair where:
- `fileNum`: Identifies the source file (e.g., SSTable file number)  
- `key`: Unique identifier within the file (e.g., block offset)
- Keys are unique for the lifetime of the cache instance due to immutable files and non-reused file numbers


## Architecture Overview

### Concurrent Dynamic-Sized Hash Tables

The hash table implementation is based on **Dynamic-Sized Nonblocking Hash Tables** research, providing:
- Lock-free bucket operations during normal access
- Automatic resizing when load thresholds are exceeded
- Non-blocking reads during resize operations
- Sorted bucket contents for efficient lookups

#### Set Operation Flow:
```
Set(fileNum, key, value):
   1. hash(fileNum, key) → bucket_id
   2. Lazy load the bucket with bucket_id in current state
   3. Insert/update (fileNum,key,value) in sorted order within bucket
   4. Update LRU cache and handle memory pressure
   5. Trigger resize if overflow thresholds exceeded
```

#### Get Operation Flow:
```
Get(fileNum, key):
   1. hash(fileNum, key) → bucket_id  
   2. Lazy load the bucket with bucket_id in current state
   3. Binary search within sorted bucket for key
   4. Increment reference count and return LazyValue
   5. Update LRU position for cache hit
```

### Dynamic Resizing

Whenever the bucket is either overloaded or under-utilised, the current state of buckets start resizing itself on the background. 
During the bucket is in the midst of re-distributing its nodes to the other buckets in the new state, the bucket is "frozen", meaning
all requests to that bucket will be deferred until a bucket finishes its re-distribution work

Since the dynamic redistribution of nodes happen on the background, therefore, in every request from client, the target bucket
always must be lazy loaded before serving

#### Growth Operation
When buckets become overloaded (> 32 entries) or global thresholds exceeded:

```
Original State (N buckets):        After Growth (2N buckets):
+--------------+------+            +--------------+------+
|   bucket 1   | data |            |   bucket 1   | data |  
+--------------+------+            +--------------+------+
|    ...       |      |            |    ...       |      |
+--------------+------+            +--------------+------+ 
|   bucket I   | data | -------->  |   bucket I   | data'|  ← Redistributed
+--------------+------+   |        +--------------+------+
|    ...       |      |   |        |    ...       |      |
+--------------+------+   |        +--------------+------+
|   bucket N   | data |    ----->  |  bucket I+N  | data"|  ← New location
+--------------+------+            +--------------+------+
                                   |    ...       |      |
                                   +--------------+------+
                                   |  bucket 2N   | data |
                                   +--------------+------+
```

#### Shrink Operation  
When buckets become underutilized (< N/2 total entries):

```
Original State (N buckets):        After Shrink (N/2 buckets):
+--------------+------+            +--------------+------+
|   bucket 1   | data |            |   bucket 1   | data |
+--------------+------+            +--------------+------+
|    ...       |      |            |    ...       |      |
+--------------+------+            +--------------+------+
|   bucket I   | data | -------->  |   bucket I   | merged|  ← Combined data
+--------------+------+    |       +--------------+------+
|    ...       |      |    |       |    ...       |      |
+--------------+------+    |       +--------------+------+
| bucket N/2+I | data | ---        |  bucket N/2  | data |
+--------------+------+            +--------------+------+
|    ...       |      |
+--------------+------+
|   bucket N   | data |
+--------------+------+
```

## Cache Replacement Policies

### LRU (Least Recently Used) - **Implemented**

The LRU implementation uses a doubly-linked list to track access patterns:

- **Structure**: Circular doubly-linked list with dummy head node
- **Promotion**: Recently accessed items move to front of list  
- **Eviction**: Items are removed from the tail when capacity exceeded
- **Thread Safety**: Protected by mutex during list operations
- **Memory Tracking**: Automatic size tracking with atomic operations


### Clock-Pro - **Planned**

Clock-Pro is a patent-free alternative to Adaptive Replacement Cache (ARC):
- Approximation of LIRS (Low Inter-reference Recency Set) algorithm
- Better performance than LRU for mixed access patterns  
- Handles both temporal and spatial locality
- Reference: [USENIX 2005 Paper](http://static.usenix.org/event/usenix05/tech/general/full_papers/jiang/jiang_html/html.html)


## Usage Examples

### Basic Usage

```go
package main

import (
    "fmt"
    cache "path/to/go-block-cache"
)

func main() {
    // Create cache with LRU policy and 10MB capacity
    blockCache := cache.NewMap(
        cache.WithCacheType(cache.LRU),
        cache.WithMaxSize(10 * 1024 * 1024), // 10MB
    )
    defer blockCache.Close(false)

    // Store a block
    fileNum, blockOffset := uint64(1), uint64(1024)
    blockData := []byte("block content...")
    
    success := blockCache.Set(fileNum, blockOffset, blockData)
    if !success {
        fmt.Println("Failed to cache block")
        return
    }

    // Retrieve the block
    lazyValue, found := blockCache.Get(fileNum, blockOffset)
    if !found {
        fmt.Println("Block not found in cache")
        return
    }
    defer lazyValue.Release() // Important: always release!

    // Access the actual data
    cachedData := lazyValue.Load()
    fmt.Printf("Retrieved block: %s\n", string(cachedData))
}
```

### High-Concurrency Pattern

```go
func workerRoutine(cache IBlockCache, fileNum uint64, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for i := 0; i < 1000; i++ {
        blockOffset := uint64(i * 4096) // 4KB blocks
        data := make([]byte, 4096)
        
        // Store block
        cache.Set(fileNum, blockOffset, data)
        
        // Immediately try to read it back
        if lazyValue, ok := cache.Get(fileNum, blockOffset); ok {
            // Use the data...
            _ = lazyValue.Load()
            lazyValue.Release()
        }
        // Note: May miss due to immediate LRU eviction under memory pressure
    }
}
```