package prefixbytescodex

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Encoder_Handcrafted_Size(t *testing.T) {
	input := [][]byte{
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 82, 101, 109, 32, 111, 109, 110, 105, 115, 32, 112, 111, 114, 114, 111, 32, 110, 101, 115, 99, 105, 117, 110, 116, 32, 99, 111, 110, 115, 101, 113, 117, 117, 110, 116, 117, 114, 32, 109, 97, 103, 110, 105, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 111, 114, 101, 32, 101, 115, 116, 32, 97, 110, 105, 109, 105, 32, 97, 115, 112, 101, 114, 110, 67, 117, 109, 32, 100, 117, 99, 105, 109, 117, 115, 32, 101, 120, 32, 100, 111, 108, 111, 114, 32, 109, 97, 105, 69, 116, 32, 114, 101, 109, 32, 118, 111, 108, 117, 112, 116, 81, 117, 111, 115, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 117, 116, 32, 110, 111, 110, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 114, 97, 116, 105, 111, 110, 101, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 111, 114, 101, 32, 101, 115, 116, 32, 97, 110, 105, 109, 105, 32, 97, 115, 112, 101, 114, 110, 67, 117, 109, 32, 100, 117, 99, 105, 109, 117, 115, 32, 101, 120, 32, 100, 111, 108, 111, 114, 32, 109, 97, 105, 69, 116, 32, 114, 101, 109, 32, 118, 111, 108, 117, 112, 116, 81, 117, 111, 115, 80, 101, 114, 115, 112, 105, 99, 105, 97, 116, 105, 115, 32, 116, 101, 109, 112, 111, 114, 97, 32, 110, 97, 109, 32, 115, 105, 110, 116, 32, 118, 101, 114, 111, 32, 109, 105, 110, 105, 109, 97, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 111, 114, 101, 32, 101, 115, 116, 32, 97, 110, 105, 109, 105, 32, 97, 115, 112, 101, 114, 110, 67, 117, 109, 32, 100, 117, 99, 105, 109, 117, 115, 32, 101, 120, 32, 100, 111, 108, 111, 114, 32, 109, 97, 105, 69, 116, 32, 114, 101, 109, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 118, 101, 114, 105, 116, 97, 116, 105, 115, 32, 101, 116, 32, 114, 101, 112, 117, 100, 105, 97, 110, 100, 97, 101, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 111, 114, 101, 32, 101, 115, 116, 32, 97, 110, 105, 109, 105, 32, 97, 115, 112, 101, 114, 110, 67, 117, 109, 32, 100, 117, 99, 105, 109, 117, 115, 32, 101, 120, 32, 100, 111, 108, 111, 114, 32, 109, 97, 105, 111, 114, 101, 115, 32, 100, 105, 103, 110, 105, 115, 115, 105, 109, 111, 115, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 78, 101, 113, 117, 101, 32, 113, 117, 105, 98, 117, 115, 100, 97, 109, 32, 108, 97, 98, 111, 114, 101, 32, 101, 115, 116, 32, 97, 110, 105, 109, 105, 32, 97, 115, 112, 101, 114, 110, 97, 116, 117, 114, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 109, 32, 100, 111, 108, 111, 114, 117, 68, 101, 98, 105, 116, 105, 115, 32, 118, 111, 108, 117, 112, 116, 97, 116, 101, 109, 32, 118, 101, 110, 105, 97, 109, 32, 112, 97, 114, 105, 97, 116, 117, 114, 32, 111, 99, 99, 97, 101, 99, 97, 116, 105, 32, 109, 97, 105, 111, 114, 101, 115, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 73, 110, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 114, 101, 114, 117, 109, 32, 100, 111, 108, 111, 114, 117, 109, 32, 115, 111, 108, 117, 116, 97, 32, 114, 101, 112, 117, 100, 105, 97, 110, 100, 97, 101, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 115, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 113, 117, 111, 100, 32, 114, 101, 114, 117, 109, 32, 101, 120, 101, 114, 99, 105, 116, 97, 116, 82, 101, 109, 32, 99, 111, 109, 78, 101, 109, 111, 32, 99, 111, 110, 115, 101, 113, 117, 117, 110, 116, 117, 114, 32, 99, 111, 110, 115, 101, 113, 117, 97, 116, 117, 114, 32, 105, 108, 108, 111, 32, 118, 111, 108, 117, 112, 116, 97, 116, 101, 32, 110, 117, 108, 108, 97, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 115, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 113, 117, 111, 100, 32, 114, 101, 114, 117, 109, 32, 101, 120, 101, 114, 99, 105, 116, 97, 116, 82, 101, 109, 32, 99, 111, 109, 109, 111, 100, 105, 32, 105, 108, 108, 117, 109, 32, 105, 100, 32, 114, 101, 112, 114, 101, 69, 118, 101, 110, 105, 101, 116, 32, 99, 111, 109, 109, 111, 100, 105, 32, 101, 116, 32, 97, 108, 105, 113, 117, 105, 100, 32, 109, 111, 108, 101, 115, 116, 105, 97, 101, 32, 101, 105, 117, 115, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 115, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 113, 117, 111, 100, 32, 114, 101, 114, 117, 109, 32, 101, 120, 101, 114, 99, 105, 116, 97, 116, 82, 101, 109, 32, 99, 111, 109, 109, 111, 100, 105, 32, 105, 108, 108, 117, 109, 32, 105, 100, 32, 114, 101, 112, 114, 101, 104, 101, 110, 100, 101, 114, 105, 116, 32, 118, 111, 108, 117, 112, 116, 97, 116, 117, 109, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 68, 117, 99, 105, 109, 117, 115, 32, 118, 111, 108, 117, 112, 116, 97, 115, 32, 113, 117, 111, 100, 32, 114, 101, 114, 117, 109, 32, 101, 120, 101, 114, 99, 105, 116, 97, 116, 105, 111, 110, 101, 109, 32, 112, 111, 114, 114, 111, 46, 0, 0, 0, 0, 0},
		{77, 65, 65, 117, 116, 32, 115, 101, 100, 32, 113, 117, 105, 97, 32, 115, 101, 113, 117, 105, 32, 101, 116, 32, 114, 101, 112, 117, 100, 105, 97, 110, 100, 77, 111, 108, 101, 115, 116, 105, 97, 101, 32, 114, 101, 112, 101, 108, 108, 101, 110, 100, 117, 115, 32, 100, 111, 108, 111, 114, 101, 109, 113, 117, 101, 32, 105, 110, 118, 101, 110, 116, 111, 114, 101, 32, 97, 100, 32, 110, 111, 110, 46, 0, 0, 0, 0, 0},
	}

	enc := &PrefixBytesEncoder{
		BundleSize: 16,
	}
	enc.Init()

	for i := range 12 {
		enc.Append(input[i])
	}

	// encode
	offset := uint32(0)
	buf := make([]byte, enc.Size(offset))

	for i := 12; i < len(input); i++ {
		enc.Append(input[i])
	}

	offset = enc.Finish(12, offset, buf)
	require.GreaterOrEqual(t, uint32(len(buf)), offset)
	// fmt.Println(uint32(len(buf)), offset)
}

func Test_Encoder_Handcrafted(t *testing.T) {
	input := [][]byte{
		{0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xa},
		{0xa, 0xa, 0xa, 0xa, 0xa, 0xa, 0xb},
		{0xa, 0xa, 0xa, 0xa, 0xa, 0xb, 0xb},
		{0xa, 0xa, 0xa, 0xa, 0xa, 0xb, 0xa},
		{0xa, 0xa, 0xb, 0xb, 0xb, 0xc},
		{0xa, 0xa, 0xb, 0xb, 0xb, 0xc, 0xc},
		{0xa, 0xa, 0xb, 0xb, 0xb, 0xc, 0xd},
		{0xa, 0xa, 0xb, 0xb, 0xc, 0xe},
		{0xa, 0xa, 0xb, 0xb, 0xe},
		{0xa, 0xa, 0xb, 0xb, 0xe},
		{0xa, 0xa, 0xb, 0xb, 0xe, 0xf},
		{0xa, 0xa, 0xb, 0xb, 0xe, 0xe},
		{0xa, 0xa, 0xc, 0xd, 0xe},
		{0xa, 0xa, 0xc, 0xd, 0xf},
		{0xa, 0xa, 0xc, 0xd, 0xf},
		{0xa, 0xa, 0xc, 0xd, 0x10, 0x10},
	}

	expectedOut := []byte{
		0x4, /* bundle size */
		/* offset encoded */
		0x1,                                    /* type */
		0x2, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, /* min value */
		0x0, 0x3, 0x5, 0x7, 0x9, 0xb, 0xd, 0xf, 0x12, 0x15, 0x17, 0x1a, 0x1a, 0x1a, 0x1b, 0x1c, 0x1e, 0x1f, 0x20, 0x21, 0x23, /* 21 offsets */
		/* compressed values */
		0xa, 0xa,
		0xa, 0xa, 0xa,
		0xa, 0xa,
		0xa, 0xb,
		0xb, 0xb,
		0xb, 0xa,
		0xb, 0xb,
		0xb, 0xc,
		0xb, 0xc, 0xc,
		0xb, 0xc, 0xd,
		0xc, 0xe,
		0xb, 0xb, 0xe,
		/* nil */
		/* nil */
		0xf,
		0xe,
		0xc, 0xd,
		0xe,
		0xf,
		0xf,
		0x10, 0x10,
	}

	enc := &PrefixBytesEncoder{
		BundleSize: 4,
	}

	enc.Init()
	for _, v := range input {
		enc.Append(v)
	}
	offset := uint32(0)
	buf := make([]byte, enc.Size(offset))

	offset = enc.Finish(uint32(len(input)), offset, buf)
	buf = buf[:offset]

	assert.True(t, bytes.Equal(expectedOut, buf))
}

func Test_Codex(t *testing.T) {
	type param struct {
		desc         string
		unique       bool
		size         int
		finishedRows int
	}

	testCases := []param{
		{
			desc:         "unique, small size p1",
			size:         16,
			finishedRows: 16,
			unique:       true,
		},
		{
			desc:         "unique, small size",
			size:         20,
			finishedRows: 20,
			unique:       true,
		},
		{
			desc:         "unique, big size",
			size:         10_000,
			finishedRows: 10_000,
			unique:       true,
		},
		{
			desc:         "not unique, small size",
			size:         20,
			finishedRows: 20,
		},
		{
			desc:         "not unique, small size p1",
			size:         16,
			finishedRows: 16,
		},
		{
			desc:         "not unique, big size",
			size:         10_000,
			finishedRows: 10_000,
		},

		{
			desc:         "unique, small size, less rows",
			size:         20,
			finishedRows: 19,
			unique:       true,
		},
		{
			desc:         "unique, small size, exact 1 block, less rows",
			size:         17,
			finishedRows: 16,
			unique:       true,
		},
		{
			desc:         "unique, big size, less rows",
			size:         10_000,
			finishedRows: 9999,
			unique:       true,
		},
		{
			desc:         "not unique, small size, less rows, size=16",
			size:         16,
			finishedRows: 15,
		},
		{
			desc:         "not unique, small size, less rows, size=20",
			size:         20,
			finishedRows: 19,
		},
		{
			desc:         "not unique, big size, less rows",
			size:         10_000,
			finishedRows: 9999,
		},
	}

	enc := &PrefixBytesEncoder{
		BundleSize: 16,
	}
	enc.Init()

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			enc.Reset()
			input := generateSortedBytes(tc.size, tc.unique)
			for i := range tc.finishedRows {
				enc.Append(input[i])
			}

			// encode
			offset := uint32(0)
			buf := make([]byte, enc.Size(offset))

			for i := tc.finishedRows; i < tc.size; i++ {
				enc.Append(input[i])
			}

			offset = enc.Finish(uint32(tc.finishedRows), offset, buf)
			require.GreaterOrEqual(t, uint32(len(buf)), offset)
			buf = buf[:offset]

			// decode
			offset = 0
			d, _ := NewPrefixBytesDecoder(uint32(tc.finishedRows), offset, buf)
			dec, ok := d.(*PrefixBytesDecoder)
			require.True(t, ok, "can not assert to PrefixBytesDecoder")

			// Verify decoder.Get()
			for i := range tc.finishedRows {
				out := dec.Get(uint32(i))
				require.True(t, bytes.Equal(out, input[i]), fmt.Sprintf("input at %d-th isn't match", i))
			}

			// Verify decoder.SeekGTE()
			for i := 1; i < tc.finishedRows; i++ {
				// fmt.Println("Testing row", i, "th")
				// seek equal to the key
				rowIndex, isEqual := dec.SeekGTE(input[i], 0, int32(tc.finishedRows)-1)
				require.True(t, isEqual, fmt.Sprintf("the key index %v should be exactly matched in the codex", i))
				require.Equal(t, uint32(i), rowIndex, fmt.Sprintf("the row index of key %v should be %d but got %d", input[i], i, rowIndex))

				// seek with a smaller key
				smallerKey := getIntermediateKey(input[i-1], input[i])
				if bytes.Compare(smallerKey, input[i]) >= 0 || bytes.Compare(smallerKey, input[i-1]) <= 0 {
					continue
				}

				rowIndex, isEqual = dec.SeekGTE(smallerKey, 0, int32(tc.finishedRows)-1)
				require.False(t, isEqual, fmt.Sprintf("the found key index %v must not equal", i))
				require.Equal(t, uint32(i), rowIndex, fmt.Sprintf("the row index of key %v should be %d but got %d", smallerKey, i, rowIndex))
			}

			// key is outside of the block
			// fmt.Println("Testing bigger row", tc.finishedRows-1, "th")
			biggerKey := append(input[tc.finishedRows-1], 0x1)
			rowIndex, isEqual := dec.SeekGTE(biggerKey, 0, int32(tc.finishedRows)-1)
			require.False(t, isEqual)
			require.Equal(t, uint32(tc.finishedRows), rowIndex)
		})
	}
}

// getIntermediateKey returns a key that is lexicographically between key1 and key2
func getIntermediateKey(key1, key2 []byte) []byte {
	lcp := block.CommonPrefix(key1, key2)
	res := make([]byte, lcp+1)
	copy(res, key1[:lcp])
	if lcp == len(key1) {
		res[len(res)-1] = 0x1
	} else {
		res[len(res)-1] = key1[lcp] + 1
	}
	return res
}

// generateSortedBytes Generate list of []byte in an increasing order of key
func generateSortedBytes(size int, isUnique bool) [][]byte {
	res := make([][]byte, 0, size)

	// generate a list of key–value pairs such that adjacent keys share some common bytes.
	for i := range size {
		res = append(res, randomByte())
		if i == 0 {
			continue
		}

		if !isUnique {
			res[i] = generateKeyFromAnotherKey(res[i-1])
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return bytes.Compare(res[i], res[j]) < 0
	})

	return res
}

// generateKeyFromAnotherKey generate a random key that share some first bytes of the given key
func generateKeyFromAnotherKey(key []byte) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	mutualLength := r.Intn(len(key)-1) + 1

	randomBytes := randomByte()

	newKey := make([]byte, mutualLength+len(randomBytes))
	copy(newKey, key)
	copy(newKey[mutualLength:], randomBytes)

	return newKey
}

func randomByte() []byte {
	quote := struct {
		FixedByteList []byte `faker:"slice_len=100"`
	}{}

	err := faker.FakeData(&quote)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return quote.FixedByteList
}
