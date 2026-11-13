package common

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_BlockHandle(t *testing.T) {
	type param struct {
		name   string
		offset uint64
		length uint64
	}

	tests := []param{
		{
			name:   "empty",
			offset: 0,
			length: 0,
		},
		{
			name:   "offset is 0, length is not 0",
			offset: 0,
			length: 1000,
		},
		{
			name:   "length is 0, offset is not 0",
			offset: 1000,
			length: 0,
		},
		{
			name:   "both are not 0",
			offset: 1000,
			length: 1000,
		},
		{
			name:   "offset is 0, length is not 0, less than 1 byte",
			offset: 0,
			length: 10,
		},
		{
			name:   "length is 0, offset is not 0, less than 1 byte",
			offset: 10,
			length: 0,
		},
		{
			name:   "both are not 0, less than 1 byte",
			offset: 10,
			length: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bh := BlockHandle{Offset: tc.offset, Length: tc.length}

			buf := make([]byte, 2*binary.MaxVarintLen64)
			n := bh.EncodeInto(buf)
			assert.Greater(t, n, 0)

			expectedBh := BlockHandle{}
			expectedN := expectedBh.DecodeFrom(buf)
			assert.Equal(t, expectedN, n, "number of bytes doesn't match")
			assert.Equal(t, bh, expectedBh, "block handle doesn't match")
		})
	}
}
