package go_block_cache

import (
	"math/rand"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func Test_AddNewNode_Then_Get_Async(t *testing.T) {
	type params struct {
		desc      string
		sequences []*kv
		expected  []*kv
	}

	newKV := func(fileNum, key uint64) *kv {
		return NewKV(fileNum, key, murmur32(fileNum, key), &shard{})
	}

	tests := []params{
		{
			desc: "Add 1 node",
			sequences: []*kv{
				newKV(0, 1),
			},
			expected: []*kv{
				newKV(0, 1),
			},
		},
		{
			desc: "Add 2 nodes in ascending order",
			sequences: []*kv{
				newKV(0, 1),
				newKV(0, 2),
			},
			expected: []*kv{
				newKV(0, 1),
				newKV(0, 2),
			},
		},
		{
			desc: "Add 2 nodes in desc order",
			sequences: []*kv{
				newKV(0, 2),
				newKV(0, 1),
			},
			expected: []*kv{
				newKV(0, 1),
				newKV(0, 2),
			},
		},
		{
			desc: "Add 3 nodes in ascending order",
			sequences: []*kv{
				newKV(0, 1),
				newKV(0, 2),
				newKV(0, 3),
			},
			expected: []*kv{
				newKV(0, 1),
				newKV(0, 2),
				newKV(0, 3),
			},
		},
		{
			desc: "Add 3 nodes in desc order",
			sequences: []*kv{
				newKV(0, 3),
				newKV(0, 2),
				newKV(0, 1),
			},
			expected: []*kv{
				newKV(0, 1),
				newKV(0, 2),
				newKV(0, 3),
			},
		},
		{
			desc: "Add 3 nodes in random order",
			sequences: []*kv{
				newKV(0, 2),
				newKV(0, 3),
				newKV(0, 1),
			},
			expected: []*kv{
				newKV(0, 1),
				newKV(0, 2),
				newKV(0, 3),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			b := &bucket{state: initialized}
			dummyS := &shard{stats: *new(Stats), state: unsafe.Pointer(new(state))}
			wg := new(sync.WaitGroup)
			for _, node := range tc.sequences {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						isFrozen, _ := b.AddNewNode(node.fileNum, node.key, node.hash, dummyS)
						if !isFrozen {
							break
						}
					}
				}()
			}
			wg.Wait()

			for i := range tc.expected {
				assert.Equal(t, tc.expected[i].fileNum, b.nodes[i].fileNum)
				assert.Equal(t, tc.expected[i].key, b.nodes[i].key)

				_, node := b.Get(tc.expected[i].fileNum, tc.expected[i].key)
				assert.NotNil(t, node)
				assert.Equal(t, node.fileNum, b.nodes[i].fileNum)
				assert.Equal(t, node.key, b.nodes[i].key)
			}
		})
	}
}

func Test_AddNewNode_Then_Get_Big_Async(t *testing.T) {
	size := 100_000
	sequences := make([]*kv, size)

	newKV := func(fileNum, key uint64) *kv {
		return NewKV(fileNum, key, murmur32(fileNum, key), &shard{})
	}

	for i := range sequences {
		sequences[i] = newKV(0, uint64(i))
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for range size / 100 {
		i, j := r.Intn(size), r.Intn(size)
		sequences[i], sequences[j] = sequences[j], sequences[i]
	}

	// add nodes to the bucket
	b := &bucket{state: initialized}
	dummyS := &shard{stats: *new(Stats), state: unsafe.Pointer(new(state))}
	wg := new(sync.WaitGroup)
	for _, node := range sequences {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.AddNewNode(node.fileNum, node.key, node.hash, dummyS)
		}()
	}
	wg.Wait()

	for i := range sequences {
		assert.Equal(t, uint64(i), b.nodes[i].key)
		_, node := b.Get(0, uint64(i))
		assert.NotNil(t, node)
		assert.Equal(t, node.fileNum, b.nodes[i].fileNum)
		assert.Equal(t, node.key, b.nodes[i].key)
	}
}
