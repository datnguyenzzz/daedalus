# Adaptive Radix Tree Implementation in Go

A implementation of the Adaptive Radix Tree (ART) data structure with Go generics and concurrent access support, and it uses Optimisitc Lock decoupling method for synchronizing 

## Benchmark Review 
- Benchmarks against 3 different synchronization methods:
  - [Coarse grained lock](coarse-grained-lock-bench-results.txt)
  - [Lock decoupling](lock-decoupling-bench-results.txt)
  - [Optimistic Lock decoupling](optimistic-decoupling-lock-bench-results.txt)

## Usage

```go
import (
    "context"
    art "github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree"
)

func main() {
    // Create a new tree with string values
    ctx := context.Background()
    tree := art.NewTree[string](ctx)
    
    // Insert key-value pairs
    key1 := []byte("hello")
    tree.Insert(ctx, key1, "world")
    
    // Retrieve values
    value, err := tree.Get(ctx, key1)
    if err == nil {
        fmt.Println(value) // Outputs: world
    }
    
    // Delete entries
    oldValue, err := tree.Delete(ctx, key1)
    
    // Walk the tree in order
    tree.Walk(ctx, func(ctx context.Context, key []byte, value string) error {
        fmt.Printf("Key: %s, Value: %s\n", key, value)
        return nil
    })
}
```

## Referrences
- https://15721.courses.cs.cmu.edu/spring2017/papers/08-oltpindexes2/leis-damon2016.pdf
- https://db.in.tum.de/~leis/papers/ART.pdf
