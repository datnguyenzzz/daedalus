package bitmapcodex

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Codex(t *testing.T) {
	type param struct {
		desc string
		size int
		lim  uint32
	}

	tests := []param{
		{
			desc: "small, lim=64",
			size: 5,
			lim:  64,
		},
		{
			desc: "medium, lim=64",
			size: 30,
			lim:  64,
		},
		{
			desc: "big, lim=64",
			size: 64,
			lim:  64,
		},
		{
			desc: "small, lim=100",
			size: 5,
			lim:  100,
		},
		{
			desc: "medium, lim=100",
			size: 50,
			lim:  100,
		},
		{
			desc: "big, lim=100",
			size: 100,
			lim:  100,
		},
		{
			desc: "small, lim=99_999_991",
			size: 10,
			lim:  99_999_991,
		},
		{
			desc: "medium, lim=99_999_991",
			size: 1_000,
			lim:  99_999_991,
		},
		{
			desc: "big, lim=99_999_991",
			size: 100_000,
			lim:  99_999_991,
		},
		{
			desc: "small, lim=10_000_000",
			size: 10,
			lim:  10_000_000,
		},
		{
			desc: "medium, lim=10_000_000",
			size: 1_000,
			lim:  10_000_000,
		},
		{
			desc: "big, lim=10_000_000",
			size: 100_000,
			lim:  10_000_000,
		},
	}

	bitmap := &BitmapEncoder{}
	bitmap.Init()

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			input := genInput(tc.size, int32(tc.lim))
			bitmap.Reset()

			for _, x := range input {
				bitmap.Append(uint32(x))
			}

			offset := uint32(0)
			buf := make([]byte, bitmap.Size(offset))
			offset = bitmap.Finish(tc.lim, offset, buf)
			buf = buf[:offset]

			// decode and verify
			offset = 0
			d, _ := NewBitmapDecoder(tc.lim, offset, buf)
			dec, ok := d.(*BitmapDecoder)
			require.True(t, ok, "failed to assert to BitmapDecoder")

			for i, x := range input {
				// get
				found := dec.Get(uint32(x))

				expected := uint32(1)
				if uint32(x) == tc.lim {
					expected = uint32(0)
				}

				require.Equal(t, expected, found, fmt.Sprintf("%d is mismatched when getting", x))

				// seekGTE
				if x-1 <= 0 || expected == uint32(0) || dec.Get(uint32(x-1)) == uint32(1) {
					continue
				}

				foundX, eq := dec.SeekGTE(uint32(x), 0, int32(tc.lim-1))
				require.Equal(t, uint32(x), foundX, fmt.Sprintf("failed at %d-th test", i))
				require.True(t, eq, fmt.Sprintf("failed at %d-th test", i))

				foundX, eq = dec.SeekGTE(uint32(x-1), 0, int32(tc.lim-1))
				require.Equal(t, uint32(x), foundX, fmt.Sprintf("failed at %d-th test", i))
				require.False(t, eq, fmt.Sprintf("failed at %d-th test", i))
			}

			foundX, eq := dec.SeekGTE(tc.lim, 0, int32(tc.lim-1))
			require.Equal(t, tc.lim, foundX)
			require.False(t, eq)
		})
	}
}

func genInput(size int, lim int32) []int32 {
	res := make([]int32, 0, size)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for range size - 1 {
		res = append(res, r.Int31n(lim))
	}

	res = append(res, lim)

	return res
}
