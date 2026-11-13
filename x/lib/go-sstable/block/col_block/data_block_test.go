package col_block_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"testing"
	"time"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	colblock "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/require"
)

const (
	suffixLen = 5
)

type mvccComparer struct {
	nogodb_common.DefaultComparer
}

func newMvccComparer() *mvccComparer {
	return &mvccComparer{
		DefaultComparer: *nogodb_common.NewComparer(),
	}
}

func (c *mvccComparer) Split(b []byte) int {
	return len(b) - suffixLen
}

var _ nogodb_common.IComparer = (*mvccComparer)(nil)

func Test_iterating_over_a_block(t *testing.T) {
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
	writer := colblock.NewDataBlockWriter(mvccComparer)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// reset writer
			writer.Reset()

			// Prep input data
			userKeys := genInput(tc.size, tc.sharedPrefixLen, true)
			keys := make([]nogodb_common.InternalKey, 0, len(userKeys))
			values := make([][]byte, 0, len(userKeys))
			for i, userKey := range userKeys {
				keys = append(keys, nogodb_common.MakeKey(userKey, nogodb_common.SeqNum(i), nogodb_common.KeyKindSet))
				values = append(values, randomByte())
			}

			// Write data
			for i := range keys {
				writer.Add(keys[i], values[i])
			}

			estSize := int(writer.Size())
			data := writer.Finish(uint32(len(keys)), estSize)

			// Create iterator
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(data))
			lz.SetBufferValue(data)

			iter := colblock.NewDataBlockIter(bp, mvccComparer, &lz)

			// Verify iterating over a block
			i := 0
			for kv := iter.First(); kv != nil; kv = iter.Next() {
				assertKv(t, i, "Next", kv, keys[i].UserKey, values[i])
				i += 1
			}

			i = len(keys) - 1
			for kv := iter.Last(); kv != nil; kv = iter.Prev() {
				assertKv(t, i, "Last", kv, keys[i].UserKey, values[i])
				i -= 1
			}
		})
	}
}

func Test_seeking_on_data_block(t *testing.T) {
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
	writer := colblock.NewDataBlockWriter(mvccComparer)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// reset writer
			writer.Reset()

			// Prep input data
			userKeys := genInput(tc.size, tc.sharedPrefixLen, true)
			keys := make([]nogodb_common.InternalKey, 0, len(userKeys))
			values := make([][]byte, 0, len(userKeys))
			for i, userKey := range userKeys {
				keys = append(keys, nogodb_common.MakeKey(userKey, nogodb_common.SeqNum(i), nogodb_common.KeyKindSet))
				values = append(values, randomByte())
			}

			// Write data
			for i := range keys {
				writer.Add(keys[i], values[i])
			}

			estSize := int(writer.Size())
			data := writer.Finish(uint32(len(keys)), estSize)

			// Create iterator
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(data))
			lz.SetBufferValue(data)

			iter := colblock.NewDataBlockIter(bp, mvccComparer, &lz)

			// 5 byte for the suffix
			kv := iter.SeekGTE([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0})
			assertKv(t, 0, "SeekGTE", kv, keys[0].UserKey, values[0])

			for i := 1; i < len(keys); i++ {
				// fmt.Println("testing", i, "-th")
				// seeking GTE directly with a key
				kv = iter.SeekGTE(keys[i].UserKey)
				assertKv(t, i, "SeekGTE strict", kv, keys[i].UserKey, values[i])

				// seeking GTE with a smaller key
				smallerKey := make([]byte, len(keys[i-1].UserKey))
				copy(smallerKey, keys[i-1].UserKey)
				smallerKey[len(smallerKey)-1] += 0x1

				if bytes.Compare(smallerKey, keys[i].UserKey) <= 0 {
					kv = iter.SeekGTE(smallerKey)
					assertKv(t, i, "SeekGTE /w smaller", kv, keys[i].UserKey, values[i])
				}

				// seeking LTE directly with a key
				kv = iter.SeekLTE(keys[i].UserKey)
				assertKv(t, i, "SeekLTE strict", kv, keys[i].UserKey, values[i])

				// seeking LTE with a bigger key
				biggerKey := make([]byte, len(keys[i].UserKey))
				copy(biggerKey, keys[i].UserKey)
				biggerKey[len(biggerKey)-1] += 0x1

				if i+1 < len(keys) && bytes.Compare(biggerKey, keys[i+1].UserKey) < 0 {
					kv = iter.SeekLTE(biggerKey)
					assertKv(t, i, "SeekLTE /w bigger", kv, keys[i].UserKey, values[i])
				}
			}
		})
	}
}

func assertKv(t *testing.T, i int, op string, foundKv *nogodb_common.InternalKV, expectedKey []byte, expectedValue []byte) {
	k, v := foundKv.K, foundKv.V.Value()
	require.Equal(t, expectedKey, k.UserKey, fmt.Sprintf("[%s]: key is mismatch at %d-th row", op, i))
	require.Zero(t, bytes.Compare(expectedValue, v), fmt.Sprintf("[%s]: value is mismatch at %d-th row", op, i))
}

// genInput a key is made of [prefix_byte][[5]byte suffix]
// a [prefix] is sorted in an increasing order.
// If key shared the same [prefix], [suffix] is in an increasing order
func genInput(size int, sharedPrefix int, withSuffix bool) [][]byte {
	res := make([][]byte, 0, size)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	prefix := randomByte()[:sharedPrefix]

	for len(res) < size {
		var c int
		if len(res)%5 == 1 || size-len(res) == 1 {
			c = 1
		} else {
			c = r.Intn(min(10, size-len(res)-1)) + 1
		}

		commonPrefix := randomByte()[:r.Intn(19)+1]
		for range c {
			key := slices.Concat(prefix, commonPrefix)
			res = append(res, key)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return bytes.Compare(res[i], res[j]) < 0
	})

	// append suffix to each key
	if withSuffix {
		for i := 0; i < len(res); {
			j := i
			for ; j < len(res) && bytes.Equal(res[i], res[j]); j++ {
			}
			for k := i; k < j; k++ {
				suffix := make([]byte, suffixLen)
				binary.LittleEndian.PutUint32(suffix, uint32(k-i))
				res[k] = slices.Concat(res[k], suffix)
			}

			i = j
		}
	}

	return res
}

func randomByte() []byte {
	quote := struct {
		FixedByteList []byte `faker:"slice_len=20"`
	}{}

	err := faker.FakeData(&quote)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return quote.FixedByteList
}
