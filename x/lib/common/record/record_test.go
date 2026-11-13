package record

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWriter(
	t *testing.T,
	reset func(),
	gen func() (string, bool),
) {
	buf := new(bytes.Buffer)
	reset()

	wr := NewWriter(buf)

	for {
		s, ok := gen()
		if !ok {
			break
		}

		w, err := wr.Next() // last block even don't have data, we still increase w.end = w.bgein + 11
		require.Nil(t, err)
		_, err = w.Write([]byte(s))
		require.Nil(t, err)
	}

	err := wr.Close()
	require.Nil(t, err)

	reset()

	r := NewReader(buf)
	for {
		s, ok := gen()
		if !ok {
			break
		}

		rr, err := r.Next()
		require.Nil(t, err)

		x, err := io.ReadAll(rr)
		require.Nil(t, err)

		require.Equal(t, string(x), s)
	}

	_, err = r.Next()
	assert.ErrorIs(t, io.EOF, err)
}

func Test_Happycase(t *testing.T) {
	testLiterals(t, []string{
		strings.Repeat("a", 1000),
		strings.Repeat("b", 90000),
		strings.Repeat("c", 8000),
	})
}

func Test_HappyCase_Near_Edge(t *testing.T) {
	for i := BlockSize - 11; i < BlockSize+11; i++ {
		s0 := strings.Repeat("a", i)
		for j := BlockSize - 11; j < BlockSize+11; j++ {
			s1 := strings.Repeat("b", i)

			testLiterals(t, []string{s0, s1})
			testLiterals(t, []string{s0, "", s1})
			testLiterals(t, []string{s0, "x", s1})
		}
	}
}

func Test_Flush(t *testing.T) {
	buf := new(bytes.Buffer)
	wr := NewWriter(buf)

	// write 2 tiny records, they should be still in the buffer
	w0, _ := wr.Next()
	w0.Write([]byte{0x1})
	w1, _ := wr.Next()
	w1.Write([]byte{0x1, 0x1})

	require.Equal(t, 0, buf.Len())

	// flush 2 records, should be len(buf) = 7 * 2 + 1 + 2
	require.NoError(t, wr.Flush())
	require.Equal(t, 17, buf.Len())

	// write 1 more, but still not large enough to fill the current block
	// so it still must be in the buffer
	w2, _ := wr.Next()
	w2.Write([]byte(strings.Repeat("a", 10_000)))
	require.Equal(t, 17, buf.Len())

	// flush 1 record, should be len(buf) = 7 + 25 + 10_000
	require.NoError(t, wr.Flush())
	require.Equal(t, 10024, buf.Len())

	// write a big one that complete the current block
	w3, _ := wr.Next()
	w3.Write([]byte(strings.Repeat("a", 40_000)))
	require.Equal(t, 32*1024, buf.Len())

	// flush should get up to: 10024 + 2*7 + 40_000
	require.NoError(t, wr.Flush())
	require.Equal(t, 50038, buf.Len())

	expecteds := []int64{1, 2, 10_000, 40_000}
	rr := NewReader(buf)
	for _, expected := range expecteds {
		r, _ := rr.Next()
		n, err := io.Copy(io.Discard, r)
		require.NoError(t, err)
		require.Equal(t, expected, n)
	}
}

func testLiterals(t *testing.T, s []string) {
	var i int

	reset := func() {
		i = 0
	}

	gen := func() (string, bool) {
		if i == len(s) {
			return "", false
		}

		i += 1
		return s[i-1], true
	}

	testWriter(t, reset, gen)
}

func Benchmark_Record_write(b *testing.B) {
	for _, size := range []int{8, 16, 32, 64, 256, 1028, 4096, 65_536} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			wr := NewWriter(io.Discard)
			defer wr.Close()

			buf := make([]byte, size)
			b.SetBytes(int64(len(buf)))

			b.ResetTimer()
			for b.Loop() {
				w, _ := wr.Next()
				if _, err := w.Write(buf); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
		})
	}
}
