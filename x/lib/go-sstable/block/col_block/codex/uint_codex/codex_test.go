package uintcodex

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
)

type param struct {
	desc     string
	width    byte
	rows     uint32
	allEqual bool

	expectedSize uint32
}

func Test_Uint_Codex_uint8(t *testing.T) {
	testCases := []param{
		{
			desc:         "small__1-byte width codex",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 10*1,
		},
		{
			desc:         "big__1-byte width codex",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*1,
		},

		{
			desc:         "small__1-byte width codex, all's equal",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "big__1-byte width codex, all's equal",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
	}
	uint_codex_test[uint8](t, testCases)
}

func Test_Uint_Codex_uint16(t *testing.T) {
	testCases := []param{
		{
			desc:         "small__1-byte width codex",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 10*1,
		},
		{
			desc:         "small__2-byte width codex",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 10*2,
		},
		{
			desc:         "big__1-byte width codex",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*1,
		},
		{
			desc:         "big__2-byte width codex",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*2,
		},

		{
			desc:         "small__1-byte width codex, all's equal",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "small__2-byte width codex, all's equal",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
		{
			desc:         "big__1-byte width codex, all's equal",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "big__2-byte width codex, all's equal",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
	}
	uint_codex_test[uint16](t, testCases)
}

func Test_Uint_Codex_uint32(t *testing.T) {
	testCases := []param{
		{
			desc:         "small__1-byte width codex",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 10*1,
		},
		{
			desc:         "small__2-byte width codex",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 10*2,
		},
		{
			desc:         "small__4-byte width codex",
			width:        4,
			rows:         10,
			expectedSize: 1 + 8 + 10*4,
		},
		{
			desc:         "big__1-byte width codex",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*1,
		},
		{
			desc:         "big__2-byte width codex",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*2,
		},
		{
			desc:         "big__4-byte width codex",
			width:        4,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*4,
		},

		{
			desc:         "small__1-byte width codex, all's equal",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "small__2-byte width codex, all's equal",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
		{
			desc:         "small__4-byte width codex, all's equal",
			width:        4,
			rows:         10,
			expectedSize: 1 + 8 + 0*4,
			allEqual:     true,
		},
		{
			desc:         "big__1-byte width codex, all's equal",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "big__2-byte width codex, all's equal",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
		{
			desc:         "big__4-byte width codex, all's equal",
			width:        4,
			rows:         3000,
			expectedSize: 1 + 8 + 0*4,
			allEqual:     true,
		},
	}
	uint_codex_test[uint32](t, testCases)
}

func Test_Uint_Codex_uint64(t *testing.T) {
	testCases := []param{
		{
			desc:         "small__1-byte width codex",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 10*1,
		},
		{
			desc:         "small__2-byte width codex",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 10*2,
		},
		{
			desc:         "small__4-byte width codex",
			width:        4,
			rows:         10,
			expectedSize: 1 + 8 + 10*4,
		},
		{
			desc:         "small__8-byte width codex",
			width:        8,
			rows:         10,
			expectedSize: 1 + 8 + 10*8,
		},
		{
			desc:         "big__1-byte width codex",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*1,
		},
		{
			desc:         "big__2-byte width codex",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*2,
		},
		{
			desc:         "big__4-byte width codex",
			width:        4,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*4,
		},
		{
			desc:         "big__8-byte width codex",
			width:        8,
			rows:         3000,
			expectedSize: 1 + 8 + 3000*8,
		},

		{
			desc:         "small__1-byte width codex, all's equal",
			width:        1,
			rows:         10,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "small__2-byte width codex, all's equal",
			width:        2,
			rows:         10,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
		{
			desc:         "small__4-byte width codex, all's equal",
			width:        4,
			rows:         10,
			expectedSize: 1 + 8 + 0*4,
			allEqual:     true,
		},
		{
			desc:         "small__8-byte width codex, all's equal",
			width:        8,
			rows:         10,
			expectedSize: 1 + 8 + 0*8,
			allEqual:     true,
		},
		{
			desc:         "big__1-byte width codex, all's equal",
			width:        1,
			rows:         3000,
			expectedSize: 1 + 8 + 0*1,
			allEqual:     true,
		},
		{
			desc:         "big__2-byte width codex, all's equal",
			width:        2,
			rows:         3000,
			expectedSize: 1 + 8 + 0*2,
			allEqual:     true,
		},
		{
			desc:         "big__4-byte width codex, all's equal",
			width:        4,
			rows:         3000,
			expectedSize: 1 + 8 + 0*4,
			allEqual:     true,
		},
		{
			desc:         "big__8-byte width codex, all's equal",
			width:        8,
			rows:         3000,
			expectedSize: 1 + 8 + 0*8,
			allEqual:     true,
		},
	}
	uint_codex_test[uint64](t, testCases)
}

func uint_codex_test[T codex.UintType](t *testing.T, testCases []param) {
	enc := new(UintEncoder[T])
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			enc.Reset()
			values := generateValues[T](tc.width, tc.rows, tc.allEqual)
			for _, v := range values {
				enc.Append(v)
			}

			assert.Equal(t, tc.expectedSize, enc.Size(0), "expected size doesn't match")

			// Encode
			offset := uint32(0)
			buf := make([]byte, enc.Size(offset)+1) // need reserve 1 un-used byte
			nextOffset := enc.Finish(uint32(len(values)), offset, buf)
			assert.Equal(t, tc.expectedSize, nextOffset, "next offset after encoding doesn't match")

			// Decode
			dec, nextOffset := NewUintDecoder[T](tc.rows, offset, buf)
			assert.Equal(t, tc.expectedSize, nextOffset, "next offset after decoding doesn't match")
			for i := range tc.rows {
				val := dec.Get(i)
				require.Equal(t, values[i], val, fmt.Sprintf("failed on row %d-th", i))
			}
		})
	}
}

func generateValues[T codex.UintType](width byte, rows uint32, allEqual bool) []T {
	res := make([]T, rows)
	if allEqual {
		for i := range res {
			res[i] = ^T(0)
		}

		return res
	}

	maxV := ^T(0)
	minV := maxV - (1<<(8*width) - 1)
	res[0] = maxV
	res[1] = minV
	for i := 2; i < len(res); i++ {
		res[i] = minV + T(rand.Uint64N(uint64(maxV-minV)))
	}

	return res
}
