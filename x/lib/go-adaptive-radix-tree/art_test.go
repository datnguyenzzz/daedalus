//go:build !race

package go_adaptive_radix_tree

// NOTE: golang race detector won't be able to recognize correctness of the optimistic locking
// and will report races if tests are executed with -race flag

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func Test_preorder_art_str_InsertAndRemoveNode_sync(t *testing.T) {
	type param struct {
		desc             string
		actions          []internal.TreeAction[string]
		expectedPreorder []internal.INode[string]
	}

	var dicts []string
	for range 20 {
		dicts = append(dicts, internal.RandomQuote())
	}

	testList := []param{
		{
			desc: "Happy case #1 - Insert exactly 1 key",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("Hello Go Lovers!!"),
						Value: dicts[0],
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.NewLeafWithKV[string](
					context.Background(),
					[]byte("Hello Go Lovers!!"),
					dicts[0],
				),
			},
		},
		{
			desc: "Happy case #2 - Insert a key and remove it",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("Hello Go Lovers!!"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("Hello Go Lovers!!"),
					},
				},
			},
			expectedPreorder: []internal.INode[string]{nil},
		},
		{
			desc: "Happy case #3 - Insert only",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir3/fileA"),
						Value: dicts[1],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileA"),
						Value: dicts[2],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[6],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileB"),
						Value: dicts[7],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileA"),
						Value: dicts[3],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileB"),
						Value: dicts[4],
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.SeedNode4[string](context.Background(), []byte("root/")),
				internal.SeedNode4[string](context.Background(), []byte("ir")),
				internal.SeedNode4[string](context.Background(), []byte("/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileA"), dicts[6]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileB"), dicts[7]),
				internal.SeedNode4[string](context.Background(), []byte("/")),
				internal.SeedNode4[string](context.Background(), []byte("ir3/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileA"), dicts[3]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileB"), dicts[4]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/fileA"), dicts[2]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir3/fileA"), dicts[1]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/fileA"), dicts[0]),
			},
		},
		{
			desc: "Happy case #4 - Insert only same as case #3 but with the shuffled the action orders, however the output should be remain",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileB"),
						Value: dicts[7],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir3/fileA"),
						Value: dicts[1],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileA"),
						Value: dicts[2],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileB"),
						Value: dicts[4],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileA"),
						Value: dicts[3],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[6],
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.SeedNode4[string](context.Background(), []byte("root/")),
				internal.SeedNode4[string](context.Background(), []byte("ir")),
				internal.SeedNode4[string](context.Background(), []byte("/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileA"), dicts[6]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileB"), dicts[7]),
				internal.SeedNode4[string](context.Background(), []byte("/")),
				internal.SeedNode4[string](context.Background(), []byte("ir3/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileA"), dicts[3]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileB"), dicts[4]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/fileA"), dicts[2]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir3/fileA"), dicts[1]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/fileA"), dicts[0]),
			},
		},
		{
			desc: "Happy case #5 - Insert only same as case #4 but with the shuffled the action orders, however the output should be remain",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileA"),
						Value: dicts[3],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[6],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileB"),
						Value: dicts[7],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir3/fileA"),
						Value: dicts[1],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileB"),
						Value: dicts[4],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileA"),
						Value: dicts[2],
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.SeedNode4[string](context.Background(), []byte("root/")),
				internal.SeedNode4[string](context.Background(), []byte("ir")),
				internal.SeedNode4[string](context.Background(), []byte("/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileA"), dicts[6]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileB"), dicts[7]),
				internal.SeedNode4[string](context.Background(), []byte("/")),
				internal.SeedNode4[string](context.Background(), []byte("ir3/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileA"), dicts[3]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileB"), dicts[4]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/fileA"), dicts[2]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir3/fileA"), dicts[1]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/fileA"), dicts[0]),
			},
		},
		{
			desc: "Happy case #6 - Insert and Delete",
			actions: []internal.TreeAction[string]{
				// insert and remove
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileC"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/fileC"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir3/fileA"),
						Value: dicts[1],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileA"),
						Value: dicts[2],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir1/fileA"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileB"),
						Value: dicts[7],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileB"),
						Value: dicts[11],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileC"),
						Value: dicts[10],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[6],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileA"),
						Value: dicts[3],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir2/fileB"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileB"),
						Value: dicts[4],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir1/fileC"),
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.SeedNode4[string](context.Background(), []byte("root/")),
				internal.SeedNode4[string](context.Background(), []byte("ir")),
				internal.SeedNode4[string](context.Background(), []byte("/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileA"), dicts[6]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileB"), dicts[7]),
				internal.SeedNode4[string](context.Background(), []byte("/")),
				internal.SeedNode4[string](context.Background(), []byte("ir3/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileA"), dicts[3]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileB"), dicts[4]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/fileA"), dicts[2]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir3/fileA"), dicts[1]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/fileA"), dicts[0]),
			},
		},
		{
			desc: "Happy case #7 - Insert and Delete, same as #6 but with shuffled the action orders, however the output should be remain",
			actions: []internal.TreeAction[string]{
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileB"),
						Value: dicts[4],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileC"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir3/fileA"),
						Value: dicts[1],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileA"),
						Value: dicts[2],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/fileC"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/fileB"),
						Value: dicts[11],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[0],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileC"),
						Value: dicts[10],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir1/fileA"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileB"),
						Value: dicts[7],
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir1/fileA"),
						Value: dicts[6],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir2/fileB"),
					},
				},
				{
					Kind: internal.InsertAction,
					KV: internal.KV[string]{
						Key:   []byte("root/dir2/dir3/fileA"),
						Value: dicts[3],
					},
				},
				{
					Kind: internal.RemoveAction,
					KV: internal.KV[string]{
						Key: []byte("root/dir1/fileC"),
					},
				},
			},
			expectedPreorder: []internal.INode[string]{
				internal.SeedNode4[string](context.Background(), []byte("root/")),
				internal.SeedNode4[string](context.Background(), []byte("ir")),
				internal.SeedNode4[string](context.Background(), []byte("/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileA"), dicts[6]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir1/fileB"), dicts[7]),
				internal.SeedNode4[string](context.Background(), []byte("/")),
				internal.SeedNode4[string](context.Background(), []byte("ir3/file")),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileA"), dicts[3]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/dir3/fileB"), dicts[4]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir2/fileA"), dicts[2]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/dir3/fileA"), dicts[1]),
				internal.SeedNodeLeaf[string](context.Background(), []byte("root/fileA"), dicts[0]),
			},
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			art := NewTree[string](ctx)

			for idx, action := range tc.actions {
				if action.Kind == internal.InsertAction {
					_, err := art.Insert(ctx, action.Key, action.Value)
					assert.NoError(t, err, fmt.Sprintf("shouldn't fail to insert new key, at action: %v-th", idx))
				} else {
					_, err := art.Delete(ctx, action.Key)
					assert.NoError(t, err, fmt.Sprintf("shouldn't fail to delete new key, at action: %v-th", idx))
				}
			}

			// verify the pre-ordered nodes in the tree
			internal.PreorderTraverseAndValidate(
				t, ctx, art.root,
				tc.expectedPreorder, 0, 0,
			)
		})
	}
}

func Test_search_art_str_InsertOnlyNode_async(t *testing.T) {
	type param struct {
		desc string
		size int
	}

	testList := []param{
		{
			desc: "small-size test p1, 3 totals",
			size: 3,
		},
		{
			desc: "small-size test p2, 20 totals",
			size: 20,
		},
		{
			desc: "small-size test p3, 50 totals",
			size: 50,
		},
		{
			desc: "medium-size test p1, 100 totals",
			size: 100,
		},
		{
			desc: "medium-size test p2, 1000 totals",
			size: 1000,
		},
		{
			desc: "medium-size test p3, 5000 totals",
			size: 5000,
		},
		{
			desc: "large-size test p1, 10000 totals",
			size: 10000,
		},
		{
			desc: "large-size test p2, 100_000 totals",
			size: 100_000,
		},
		{
			desc: "large-size test p3, 250_000 totals",
			size: 250_000,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			art := NewTree[string](ctx)

			// adding to-be-deleted keys into the tree and read to verify
			eg, egCtx := errgroup.WithContext(ctx)
			eg.SetLimit(20)

			expectedTotalKVMap := internal.SeedMapKVString(tc.size)

			for _, kv := range expectedTotalKVMap {
				eg.Go(func() error {
					_, err := art.Insert(egCtx, kv.Key, kv.Value)
					return err
				})
			}

			err := eg.Wait()
			assert.NoError(t, err, fmt.Sprintf("shouldn't fail to insert new key. Err: %v", err))

			testWalkCallback(t, art, expectedTotalKVMap, true)
			testWalkCallback(t, art, expectedTotalKVMap, false)

			// verify key value after deletion
			for _, kv := range expectedTotalKVMap {
				actualV, err := art.Get(ctx, kv.Key)
				assert.NoError(t, err, fmt.Sprintf("shouldn't fail to get key. Err: %v", err))
				assert.Equal(t, kv.Value, actualV, "value should be equal")
			}
		})
	}
}

func Test_search_art_str_DeleteOnlyNode_async(t *testing.T) {
	type param struct {
		desc string
		size int
	}

	testList := []param{
		{
			desc: "small-size test p1, 3 totals",
			size: 3,
		},
		{
			desc: "small-size test p2, 20 totals",
			size: 20,
		},
		{
			desc: "small-size test p3, 50 totals",
			size: 50,
		},
		{
			desc: "medium-size test p1, 100 totals",
			size: 100,
		},
		{
			desc: "medium-size test p2, 1000 totals",
			size: 1000,
		},
		{
			desc: "medium-size test p3, 5000 totals",
			size: 5000,
		},
		{
			desc: "large-size test p1, 10000 totals",
			size: 10000,
		},
		{
			desc: "large-size test p2, 100_000 totals",
			size: 100_000,
		},
		{
			desc: "large-size test p3, 250_000 totals",
			size: 250_000,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			art := NewTree[string](ctx)

			// adding to-be-deleted keys into the tree and read to verify

			expectedTotalKVMap := internal.SeedMapKVString(tc.size)

			for _, kv := range expectedTotalKVMap {
				_, err := art.Insert(ctx, kv.Key, kv.Value)
				require.NoError(t, err)
			}

			eg, egCtx := errgroup.WithContext(ctx)
			eg.SetLimit(20)

			// delete
			for _, kv := range expectedTotalKVMap {
				eg.Go(func() error {
					_, err := art.Delete(egCtx, kv.Key)
					require.NoError(t, err)
					return err
				})
			}

			// delete some non-exist keys
			for i := range expectedTotalKVMap {
				eg.Go(func() error {
					_, err := art.Delete(egCtx, fmt.Appendf(nil, "non-exist-key-%d", i))
					assert.ErrorIs(t, err, NonExist)
					return nil
				})
			}

			err := eg.Wait()
			assert.NoError(t, err, fmt.Sprintf("shouldn't fail to delete keys. Err: %v", err))

			for _, kv := range expectedTotalKVMap {
				_, err := art.Get(ctx, kv.Key)
				assert.ErrorIs(t, err, NonExist)
			}
		})
	}
}

func Test_search_art_str_InsertAndRemoveNode_async(t *testing.T) {
	type param struct {
		desc string
		size int
		// from insertActionsCount - len(expectedFinalKVMap) -> len(expectedFinalKVMap) deletion actions
		deleteActionsCount int
	}

	testList := []param{
		{
			desc:               "small-size test p1, 3 totals, 2 removes",
			size:               3,
			deleteActionsCount: 2,
		},
		{
			desc:               "small-size test p2, 20 totals, 6 removes",
			size:               20,
			deleteActionsCount: 6,
		},
		{
			desc:               "small-size test p3, 50 totals, 20 removes",
			size:               50,
			deleteActionsCount: 20,
		},
		{
			desc:               "medium-size test p1, 100 totals, 40 removes",
			size:               100,
			deleteActionsCount: 40,
		},
		{
			desc:               "medium-size test p2, 1000 totals, 400 removes",
			size:               1000,
			deleteActionsCount: 400,
		},
		{
			desc:               "medium-size test p3, 5000 totals, 1500 removes",
			size:               5000,
			deleteActionsCount: 1500,
		},
		{
			desc:               "large-size test p1, 10000 totals, 4000 removes",
			size:               10000,
			deleteActionsCount: 4000,
		},
		{
			desc:               "large-size test p2, 100_000 totals, 60_000 removes",
			size:               100_000,
			deleteActionsCount: 60_000,
		},
		{
			desc:               "large-size test p3, 250_000 totals, 150_000 removes",
			size:               250_000,
			deleteActionsCount: 150_000,
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			art := NewTree[string](ctx)

			expectedTotalKVMap := internal.SeedMapKVString(tc.size)

			// adding to-be-deleted keys into the tree and read to verify
			eg, egCtx := errgroup.WithContext(ctx)
			eg.SetLimit(20)
			for i := tc.size - tc.deleteActionsCount; i < tc.size; i++ {
				kv := expectedTotalKVMap[i]
				eg.Go(func() error {
					_, err := art.Insert(egCtx, kv.Key, kv.Value)
					return err
				})
			}

			err := eg.Wait()
			assert.NoError(t, err, fmt.Sprintf("shouldn't fail to insert new key. Err: %v", err))

			testWalkCallback(t, art, expectedTotalKVMap[tc.size-tc.deleteActionsCount:], true)
			testWalkCallback(t, art, expectedTotalKVMap[tc.size-tc.deleteActionsCount:], false)
			eg, egCtx = errgroup.WithContext(ctx)
			eg.SetLimit(20)
			// insert
			for i := range tc.size - tc.deleteActionsCount {
				kv := expectedTotalKVMap[i]
				eg.Go(func() error {
					_, err := art.Insert(egCtx, kv.Key, kv.Value)
					return err
				})
			}
			// delete
			for i := tc.size - tc.deleteActionsCount; i < tc.size; i++ {
				kv := expectedTotalKVMap[i]
				eg.Go(func() error {
					_, err := art.Delete(egCtx, kv.Key)
					return err
				})
			}

			err = eg.Wait()
			assert.NoError(t, err, fmt.Sprintf("shouldn't fail to insert new key. Err: %v", err))

			// verify key value after deletion
			for i := range tc.size - tc.deleteActionsCount {
				kv := expectedTotalKVMap[i]
				actualV, err := art.Get(ctx, kv.Key)
				assert.NoError(t, err, fmt.Sprintf("shouldn't fail to get key. Err: %v", err))
				assert.Equal(t, kv.Value, actualV, "value should be equal")
			}

			testWalkCallback(t, art, expectedTotalKVMap[:tc.size-tc.deleteActionsCount], true)
			testWalkCallback(t, art, expectedTotalKVMap[:tc.size-tc.deleteActionsCount], false)
		})
	}
}

func testWalkCallback[V any](t *testing.T, art ITree[V], inputKVs []internal.KV[V], isBackward bool) {
	outputKVs := make([]internal.KV[V], 0, len(inputKVs))

	var updateFn WalkFn[V] = func(ctx context.Context, k Key, v V) error {
		outputKVs = append(outputKVs, internal.KV[V]{
			Key: k, Value: v,
		})
		return nil
	}

	if isBackward {
		art.WalkBackwards(context.Background(), updateFn)
	} else {
		art.Walk(context.Background(), updateFn)
	}

	assert.Equal(t, len(inputKVs), len(outputKVs))

	expectedKVs := make([]internal.KV[V], len(inputKVs))
	copy(expectedKVs, inputKVs)
	sort.Slice(expectedKVs, func(i, j int) bool {
		return bytes.Compare(expectedKVs[i].Key, expectedKVs[j].Key) < 0
	})

	for i, expectedKV := range expectedKVs {
		actualKV := outputKVs[i]
		if isBackward {
			actualKV = outputKVs[len(outputKVs)-1-i]
		}
		assert.Equal(t, expectedKV.Key, actualKV.Key)
		assert.Equal(t, expectedKV.Value, actualKV.Value)
	}
}

func Test_ART_EdgeCases(t *testing.T) {
	ctx := context.Background()
	art := NewTree[string](ctx)

	t.Run("Get/Delete on empty tree", func(t *testing.T) {
		_, err := art.Get(ctx, []byte("non-existent"))
		assert.ErrorIs(t, err, NonExist)

		_, err = art.Delete(ctx, []byte("non-existent"))
		assert.ErrorIs(t, err, NonExist)
	})

	t.Run("Insert duplicate keys updates value", func(t *testing.T) {
		_, err := art.Insert(ctx, []byte("key1"), "val1")
		assert.NoError(t, err)

		oldVal, err := art.Insert(ctx, []byte("key1"), "val2")
		assert.NoError(t, err)
		assert.Equal(t, "val1", oldVal)

		val, err := art.Get(ctx, []byte("key1"))
		assert.NoError(t, err)
		assert.Equal(t, "val2", val)
	})

	t.Run("Delete non-existent key from non-empty tree", func(t *testing.T) {
		_, err := art.Delete(ctx, []byte("key2"))
		assert.ErrorIs(t, err, NonExist)
	})
}

func Test_ART_Visualize(t *testing.T) {
	ctx := context.Background()
	art := NewTree[string](ctx)

	t.Run("Visualize empty tree", func(t *testing.T) {
		assert.NotPanics(t, func() { art.Visualize(ctx) })
	})

	t.Run("Visualize tree with one leaf", func(t *testing.T) {
		art.Insert(ctx, []byte("key1"), "val1")
		assert.NotPanics(t, func() { art.Visualize(ctx) })
	})

	t.Run("Visualize small tree", func(t *testing.T) {
		art.Insert(ctx, []byte("key2"), "val2")
		art.Insert(ctx, []byte("key3"), "val3")
		assert.NotPanics(t, func() { art.Visualize(ctx) })
	})
}

func Test_ART_NodeTransitions(t *testing.T) {
	ctx := context.Background()
	art := NewTree[int](ctx)

	// Growth: 4 -> 16 -> 48 -> 256
	t.Run("Node growth and shrinkage", func(t *testing.T) {
		// Insert 256 keys with different first bytes to force node growth
		keys := make([][]byte, 256)
		for i := range 256 {
			keys[i] = []byte{byte(i)}
			_, err := art.Insert(ctx, keys[i], i)
			assert.NoError(t, err)
		}

		// Verify all keys
		for i := range 256 {
			val, err := art.Get(ctx, keys[i])
			assert.NoError(t, err)
			assert.Equal(t, i, val)
		}

		// Shrinkage: 256 -> 48 -> 16 -> 4
		for i := 255; i >= 0; i-- {
			_, err := art.Delete(ctx, keys[i])
			assert.NoError(t, err)

			// Occasionally verify remaining keys
			if i > 0 && i%64 == 0 {
				for j := range i {
					val, err := art.Get(ctx, keys[j])
					assert.NoError(t, err)
					assert.Equal(t, j, val)
				}
			}
		}

		// Tree should be empty
		_, err := art.Get(ctx, keys[0])
		assert.ErrorIs(t, err, NonExist)
	})
}

func Test_ART_PrefixHandling(t *testing.T) {
	ctx := context.Background()
	art := NewTree[string](ctx)

	type testCase struct {
		key   string
		value string
	}

	tests := []testCase{
		{"apple", "1"},
		{"banana", "2"},
		{"application", "4"},
		{"apply", "2"},
		{"app", "3"},
		{"bandana", "6"},
		{"band", "7"},
		{"cherry", "3"},
		{"date", "4"},
	}

	for _, tc := range tests {
		_, err := art.Insert(ctx, []byte(tc.key), tc.value)
		assert.NoError(t, err)
	}

	for _, tc := range tests {
		val, err := art.Get(ctx, []byte(tc.key))
		assert.NoError(t, err, "Failed to get key: %s", tc.key)
		assert.Equal(t, tc.value, val)
	}
}
