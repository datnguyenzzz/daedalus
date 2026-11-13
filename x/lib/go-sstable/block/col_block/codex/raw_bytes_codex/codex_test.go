package rawbytescodex

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_codex(t *testing.T) {
	type param struct {
		desc         string
		size         uint32
		finishedSize uint32
	}

	testCases := []param{
		{
			desc:         "small size",
			size:         5,
			finishedSize: 5,
		},
		{
			desc:         "medium size",
			size:         100,
			finishedSize: 100,
		},
		{
			desc:         "big size",
			size:         5000,
			finishedSize: 5000,
		},
		//
		{
			desc:         "small size, less rows",
			size:         5,
			finishedSize: 4,
		},
		{
			desc:         "medium size, less rows",
			size:         100,
			finishedSize: 99,
		},
		{
			desc:         "big size, less rows",
			size:         5000,
			finishedSize: 4999,
		},
	}

	enc := new(RawByteEncoder)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			enc.Reset()

			values := make([][]byte, tc.size)
			for i := range tc.size {
				values[i] = randomByte()
			}

			for _, v := range values {
				enc.Append(v)
			}

			// encode
			offset := uint32(0)
			buf := make([]byte, enc.Size(offset)+1) // need to reserve 1 unused byte
			totalSize := enc.Finish(tc.finishedSize, offset, buf)
			require.GreaterOrEqual(t, uint32(len(buf)), totalSize)

			// decode
			dec, nextOffset := NewRawBytesDecoder(tc.finishedSize, offset, buf)

			assert.Equal(t, nextOffset, totalSize)
			for i := range tc.finishedSize {
				v := values[i]
				val := dec.Get(i)
				assert.True(t, bytes.Equal(val, v))
			}
		})
	}
}

func randomByte() []byte {
	quote := struct {
		FixedByteList []byte `faker:"slice_len=1000"`
	}{}

	err := faker.FakeData(&quote)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return quote.FixedByteList
}
