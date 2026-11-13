package col_block_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	colblock "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/stretchr/testify/require"
)

func Test_iterating_over_an_index_block(t *testing.T) {
	type param struct {
		desc            string
		sharedPrefixLen int
		size            int
	}

	testCases := []param{
		{
			desc: "small, without shared prefixes",
			size: 20,
		},
		{
			desc:            "small, with shared prefixes",
			size:            20,
			sharedPrefixLen: 5,
		},
		{
			desc: "medium, without shared prefixes",
			size: 2000,
		},
		{
			desc:            "medium, with shared prefixes",
			size:            2000,
			sharedPrefixLen: 10,
		},
		{
			desc: "big, without shared prefixes",
			size: 10_000,
		},
		{
			desc:            "big, with shared prefixes",
			size:            10_000,
			sharedPrefixLen: 20,
		},
	}

	mvccComparer := newMvccComparer()
	bp := predictable_size.NewPredictablePool()
	writer := colblock.NewIndexBlockWriter()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// reset writer
			writer.Reset()

			// Prep input data
			keys := reduceDuplicate(genInput(tc.size, tc.sharedPrefixLen, false))
			blockHandles := make([]*common.BlockHandle, 0, len(keys))
			for range keys {
				blockHandles = append(blockHandles, &common.BlockHandle{
					Offset: r.Uint64(),
					Length: r.Uint64(),
				})
			}

			// Write data
			for i := range keys {
				writer.Add(keys[i], blockHandles[i])
			}

			estSize := int(writer.Size())
			data := writer.Finish(uint32(len(keys)), estSize)

			// Create iterator
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(data))
			lz.SetBufferValue(data)

			iter := colblock.NewIndexBlockIter(bp, mvccComparer, &lz)

			// Verify iterating over a block
			i := 0
			for kv := iter.First(); kv != nil; kv = iter.Next() {
				assertIndexKv(t, "Next", kv, keys[i], blockHandles[i])
				i += 1
			}

			i = len(keys) - 1
			for kv := iter.Last(); kv != nil; kv = iter.Prev() {
				assertIndexKv(t, "Prev", kv, keys[i], blockHandles[i])
				i -= 1
			}
		})
	}
}

func Test_Seeking_on_an_index_block(t *testing.T) {
	type param struct {
		desc            string
		sharedPrefixLen int
		size            int
	}

	testCases := []param{
		{
			desc: "small, without shared prefixes",
			size: 20,
		},
		{
			desc:            "small, with shared prefixes",
			size:            20,
			sharedPrefixLen: 5,
		},
		{
			desc: "medium, without shared prefixes",
			size: 2000,
		},
		{
			desc:            "medium, with shared prefixes",
			size:            2000,
			sharedPrefixLen: 10,
		},
		{
			desc: "big, without shared prefixes",
			size: 10_000,
		},
		{
			desc:            "big, with shared prefixes",
			size:            10_000,
			sharedPrefixLen: 20,
		},
	}

	mvccComparer := newMvccComparer()
	bp := predictable_size.NewPredictablePool()
	writer := colblock.NewIndexBlockWriter()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// reset writer
			writer.Reset()

			// Prep input data
			keys := reduceDuplicate(genInput(tc.size, tc.sharedPrefixLen, false))
			blockHandles := make([]*common.BlockHandle, 0, len(keys))
			for range keys {
				blockHandles = append(blockHandles, &common.BlockHandle{
					Offset: r.Uint64(),
					Length: r.Uint64(),
				})
			}

			// Write data
			for i := range keys {
				writer.Add(keys[i], blockHandles[i])
			}

			estSize := int(writer.Size())
			data := writer.Finish(uint32(len(keys)), estSize)

			// Create iterator
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(data))
			lz.SetBufferValue(data)

			iter := colblock.NewIndexBlockIter(bp, mvccComparer, &lz)

			// 5 byte for the suffix
			kv := iter.SeekGTE([]byte{0x0})
			assertIndexKv(t, "SeekGTE", kv, keys[0], blockHandles[0])

			for i := 1; i < len(keys); i++ {
				// fmt.Println("testing", i, "-th")
				// seeking GTE directly with a key
				kv = iter.SeekGTE(keys[i])
				assertIndexKv(t, "SeekGTE strict", kv, keys[i], blockHandles[i])

				// seeking GTE with a smaller key
				smallerKey := make([]byte, len(keys[i-1]))
				copy(smallerKey, keys[i-1])
				smallerKey[len(smallerKey)-1] += 0x1

				if bytes.Compare(smallerKey, keys[i]) <= 0 {
					kv = iter.SeekGTE(smallerKey)
					assertIndexKv(t, "SeekGTE /w smaller", kv, keys[i], blockHandles[i])
				}

				// seeking LTE directly with a key
				kv = iter.SeekLTE(keys[i])
				assertIndexKv(t, "SeekLTE strict", kv, keys[i], blockHandles[i])

				// seeking LTE with a bigger key
				biggerKey := make([]byte, len(keys[i]))
				copy(biggerKey, keys[i])
				biggerKey[len(biggerKey)-1] += 0x1

				if i+1 < len(keys) && bytes.Compare(biggerKey, keys[i+1]) < 0 {
					kv = iter.SeekLTE(biggerKey)
					assertIndexKv(t, "SeekLTE /w bigger", kv, keys[i], blockHandles[i])
				}
			}
		})
	}
}

func reduceDuplicate(slice [][]byte) [][]byte {
	res := make([][]byte, 0, len(slice))
	for i := 0; i < len(slice); {
		j := i
		for ; j < len(slice) && bytes.Equal(slice[i], slice[j]); j++ {
		}

		res = append(res, slice[i])
		i = j
	}

	return res
}

func assertIndexKv(
	t *testing.T,
	op string,
	actual *nogodb_common.InternalKV,
	expectedKey []byte,
	expectedBh *common.BlockHandle,
) {
	k, v := actual.K.UserKey, actual.V.Value()
	bh := &common.BlockHandle{}
	bh.DecodeFrom(v)

	require.Equal(t, expectedKey, k, fmt.Sprintf("[%s]: key is mismatch", op))
	require.Equal(t, expectedBh.Offset, bh.Offset, fmt.Sprintf("[%s]: offset is mismatch", op))
	require.Equal(t, expectedBh.Length, bh.Length, fmt.Sprintf("[%s]: length is mismatch", op))
}
