package go_blocked_bloom_filter

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func nextN(n int) int {
	switch {
	case n < 10:
		n += 1
	case n < 100:
		n += 10
	case n < 1000:
		n += 100
	default:
		n += 1000
	}
	return n
}

func TestBloomFilter_Small(t *testing.T) {
	bf := NewBloomFilter()
	writer := bf.NewWriter()
	writer.Add([]byte("hello"))
	writer.Add([]byte("world"))
	var filter []byte
	writer.Build(&filter)
	assert.True(t, bf.MayContain(filter, []byte("hello")))
	assert.True(t, bf.MayContain(filter, []byte("world")))
	assert.False(t, bf.MayContain(filter, []byte("x")))
	assert.False(t, bf.MayContain(filter, []byte("foo")))
}

func TestBloomFilter_VaryingLengths(t *testing.T) {
	var mediocre, good int
	for n := 1; n < 100_000; n = nextN(n) {
		bf := NewBloomFilter()
		writer := bf.NewWriter()
		for i := range n {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], uint32(i))
			writer.Add(b[:])
		}

		var filter []byte
		writer.Build(&filter)

		// assert must not false negative
		for i := range n {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], uint32(i))
			isIn := bf.MayContain(filter, b[:])
			assert.True(t, isIn, fmt.Sprintf("%d should be is membership, false negative is not allowed", i))
		}

		// assert the false positive rate
		var fpr float32
		for i := range 10_000 {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], uint32(i+1e9))
			if bf.MayContain(filter, b[:]) {
				fpr++
			}
		}
		fpr /= 10_000
		// as we choose defaultBitsPerKeys = 10, so the fpr should not be > 0.02
		assert.LessOrEqual(t, fpr, float32(0.02), fmt.Sprintf("false positive rate: %v%%, is too high", fpr))
		if fpr >= float32(0.0125) {
			mediocre++
		} else {
			good++
		}
	}

	// number of mediocre filter set should not larger than 20% of the good ones
	assert.Less(t, mediocre, (good+4)/5, fmt.Sprintf("mediocre (%d) is too high, it is higher than 20%% of good (%d)", mediocre, good))
}
