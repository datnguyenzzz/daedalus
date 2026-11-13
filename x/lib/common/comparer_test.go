package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComparer_Compare(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want int
	}{
		{
			name: "equal empty",
			a:    []byte{},
			b:    []byte{},
			want: 0,
		},
		{
			name: "equal non-empty",
			a:    []byte("hello"),
			b:    []byte("hello"),
			want: 0,
		},
		{
			name: "a < b",
			a:    []byte("apple"),
			b:    []byte("banana"),
			want: -1,
		},
		{
			name: "a > b",
			a:    []byte("zebra"),
			b:    []byte("yellow"),
			want: 1,
		},
		{
			name: "prefix - a < b",
			a:    []byte("foo"),
			b:    []byte("foobar"),
			want: -1,
		},
		{
			name: "prefix - a > b",
			a:    []byte("foobar"),
			b:    []byte("foo"),
			want: 1,
		},
		{
			name: "different bytes",
			a:    []byte{0x00, 0x01, 0x02},
			b:    []byte{0x00, 0x01, 0x03},
			want: -1,
		},
		{
			name: "with null bytes",
			a:    []byte("a\x00b"),
			b:    []byte("a\x00c"),
			want: -1,
		},
	}

	comparer := NewComparer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparer.Compare(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "Compare(%q, %q)", tt.a, tt.b)

			// Verify that the implementation matches bytes.Compare
			bytesCompareResult := bytes.Compare(tt.a, tt.b)
			assert.Equal(t, bytesCompareResult, got, "Should match bytes.Compare")
		})
	}
}

func TestComparer_Separator(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want []byte
	}{
		{
			name: "empty inputs",
			a:    []byte{},
			b:    []byte{},
			want: []byte{},
		},
		{
			name: "equal inputs",
			a:    []byte("hello"),
			b:    []byte("hello"),
			want: []byte("hello"),
		},
		{
			name: "a prefix of b",
			a:    []byte("foo"),
			b:    []byte("foobar"),
			want: []byte("foo"),
		},
		{
			name: "b prefix of a",
			a:    []byte("foobar"),
			b:    []byte("foo"),
			want: []byte("foobar"),
		},
		{
			name: "different at first byte",
			a:    []byte("a"),
			b:    []byte("c"),
			want: []byte("b"), // a + 1 < c
		},
		{
			name: "consecutive bytes",
			a:    []byte("apple"),
			b:    []byte("banana"),
			want: []byte("b"), // a + 1 = b
		},
		{
			name: "common prefix",
			a:    []byte("abc"),
			b:    []byte("abd"),
			want: []byte("abc"),
		},
		{
			name: "with 0xFF byte in a",
			a:    []byte{0xFF},
			b:    []byte{0xFF, 0x01},
			want: []byte{0xFF},
		},
		{
			name: "shared prefix with a[i] = 0xFF",
			a:    []byte{0x01, 0xFF, 0xFF},
			b:    []byte{0x01, 0xFF, 0xFF, 0x01},
			want: []byte{0x01, 0xFF, 0xFF},
		},
		{
			name: "a fully 0xFF",
			a:    []byte{0xFF, 0xFF, 0xFF},
			b:    []byte{0xFF, 0xFF, 0xFF, 0x01},
			want: []byte{0xFF, 0xFF, 0xFF},
		},
		{
			name: "with 0xFF at separator position",
			a:    []byte{0x01, 0xFE},
			b:    []byte{0x01, 0xFF},
			want: []byte{0x01, 0xFE},
		},
	}

	comparer := NewComparer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with empty destination
			got := comparer.Separator(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "Separator(%q, %q)", tt.a, tt.b)

			abCompare := comparer.Compare(tt.a, tt.b)
			if abCompare >= 0 {
				assert.Equal(t, got, tt.a, "In case of a >= b, the result should be equal to a")
			} else {
				// Verify separator is between a and b
				if len(tt.a) > 0 && len(tt.b) > 0 && !bytes.Equal(tt.a, tt.b) {
					aCompare := comparer.Compare(tt.a, got)
					bCompare := comparer.Compare(got, tt.b)

					assert.LessOrEqual(t, aCompare, 0, "Expected a <= separator, got a > separator for %q and %q", tt.a, got)
					assert.Less(t, bCompare, 0, "Expected separator < b, got separator >= b for %q and %q", got, tt.b)
				}
			}
		})
	}
}

func TestComparer_Successor(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		want []byte
	}{
		{
			name: "empty input",
			b:    []byte{},
			want: []byte{},
		},
		{
			name: "single byte",
			b:    []byte{0x01},
			want: []byte{0x02}, // increment the byte
		},
		{
			name: "multiple bytes",
			b:    []byte{0x01, 0x02, 0x03},
			want: []byte{0x02}, // increment the last byte
		},
		{
			name: "increment first byte",
			b:    []byte{0x01, 0xFF, 0xFF},
			want: []byte{0x02}, // increment the first non-0xFF byte
		},
		{
			name: "increment middle byte",
			b:    []byte{0xFF, 0x01, 0xFF},
			want: []byte{0xFF, 0x02}, // increment the first non-0xFF byte
		},
		{
			name: "all bytes are 0xFF",
			b:    []byte{0xFF, 0xFF, 0xFF},
			want: []byte{0xFF, 0xFF, 0xFF}, // no successor possible, return input
		},
		{
			name: "ASCII string",
			b:    []byte("hello"),
			want: []byte("i"),
		},
	}
	comparer := NewComparer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with empty destination
			got := comparer.Successor(tt.b)
			assert.Equal(t, tt.want, got, "Successor(%q)", tt.b)

			// Verify successor is greater than or equal to b
			if len(tt.b) > 0 && !bytes.Equal(tt.b, got) {
				bCompare := comparer.Compare(tt.b, got)
				assert.LessOrEqual(t, bCompare, 0, "Expected b <= successor for %q and %q", tt.b, got)
			}
		})
	}
}

func TestComparer_Interface(t *testing.T) {
	// Verify that comparer implements IComparer interface
	var _ IComparer = (*DefaultComparer)(nil)
	var _ IComparer = NewComparer()
}
