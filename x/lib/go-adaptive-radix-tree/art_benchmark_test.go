package go_adaptive_radix_tree

import (
	"context"
	"fmt"
	"testing"

	"github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func BenchmarkInsert(b *testing.B) {
	kvs := internal.SeedMapKVString(10_000_000)
	ctx := context.Background()

	concurrencies := []int{1, 10, 20}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("BenchmarkInsert-%d", concurrency), func(b *testing.B) {
			for b.Loop() {
				eg, egCtx := errgroup.WithContext(ctx)
				eg.SetLimit(concurrency)
				art := NewTree[string](egCtx)
				for _, kv := range kvs {
					eg.Go(func() error {
						_, err := art.Insert(egCtx, kv.Key, kv.Value)
						require.NoError(b, err)
						return err
					})
				}

				err := eg.Wait()
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkGet(b *testing.B) {
	kvs := internal.SeedMapKVString(10_000_000)
	ctx := context.Background()
	art := NewTree[string](ctx)
	for _, kv := range kvs {
		_, _ = art.Insert(ctx, kv.Key, kv.Value)
	}

	concurrencies := []int{1, 10, 20}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("BenchmarkGet-%d", concurrency), func(b *testing.B) {
			for b.Loop() {
				eg, egCtx := errgroup.WithContext(ctx)
				eg.SetLimit(concurrency)
				for _, kv := range kvs {
					eg.Go(func() error {
						v, err := art.Get(egCtx, kv.Key)
						assert.Equal(b, kv.Value, v)
						require.NoError(b, err)
						return err
					})
				}

				err := eg.Wait()
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkInsertAndGet(b *testing.B) {
	kvs := internal.SeedMapKVString(10_000_000)
	ctx := context.Background()
	art := NewTree[string](ctx)

	concurrencies := []int{1, 10, 20}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("BenchmarkInsertAndGet-%d", concurrency), func(b *testing.B) {
			for b.Loop() {
				eg, egCtx := errgroup.WithContext(ctx)
				eg.SetLimit(concurrency)
				for _, kv := range kvs {
					eg.Go(func() error {
						_, err := art.Insert(egCtx, kv.Key, kv.Value)
						require.NoError(b, err)
						v, err := art.Get(egCtx, kv.Key)
						assert.Equal(b, kv.Value, v)
						require.NoError(b, err)

						return nil
					})
				}

				err := eg.Wait()
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkInsertDeleteAndGet(b *testing.B) {
	kvs := internal.SeedMapKVString(10_000_000)
	ctx := context.Background()
	art := NewTree[string](ctx)

	concurrencies := []int{1, 10, 20}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("BenchmarkInsertAndGet-%d", concurrency), func(b *testing.B) {
			for b.Loop() {
				eg, egCtx := errgroup.WithContext(ctx)
				eg.SetLimit(concurrency)
				for i, kv := range kvs {
					eg.Go(func() error {
						_, err := art.Insert(egCtx, kv.Key, kv.Value)
						require.NoError(b, err)
						v, err := art.Get(egCtx, kv.Key)
						assert.Equal(b, kv.Value, v)
						require.NoError(b, err)

						if i%2 == 0 {
							_, err := art.Delete(egCtx, kv.Key)
							require.NoError(b, err)
							_, err = art.Get(egCtx, kv.Key)
							assert.ErrorIs(b, err, NonExist)
						}

						return nil
					})
				}

				err := eg.Wait()
				require.NoError(b, err)
			}
		})
	}
}
