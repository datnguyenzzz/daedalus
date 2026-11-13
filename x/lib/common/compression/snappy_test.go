package compression

import (
	"bytes"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnappyCompressor_GetType(t *testing.T) {
	compressor := &snappyCompressor{}
	assert.Equal(t, SnappyCompression, compressor.GetType())
}

func TestSnappyCompressor_Compress(t *testing.T) {
	compressor := &snappyCompressor{}

	tests := []struct {
		name string
		src  []byte
		dst  []byte
	}{
		{
			name: "empty input",
			src:  []byte{},
			dst:  make([]byte, 0, 100),
		},
		{
			name: "simple text",
			src:  []byte("hello world"),
			dst:  make([]byte, 0, 100),
		},
		{
			name: "repeated pattern",
			src:  bytes.Repeat([]byte("abc"), 1000),
			dst:  make([]byte, 0, 1000),
		},
		{
			name: "random data",
			src:  []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xff, 0xfe, 0xfd},
			dst:  make([]byte, 0, 100),
		},
		{
			name: "large text with repetition",
			src: []byte("The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog."),
			dst: make([]byte, 0, 200),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test compression
			compressed := compressor.Compress(tt.dst, tt.src)

			// Verify compression worked
			assert.NotNil(t, compressed)

			// Verify we can decompress it back using standard snappy
			decompressedLen, err := compressor.DecompressedLen(compressed)
			assert.NoError(t, err)
			decompressed := make([]byte, decompressedLen)

			err = compressor.Decompress(decompressed, compressed)
			require.NoError(t, err)
			assert.Equal(t, tt.src, decompressed)
		})
	}
}

func TestSnappyCompressor_Decompress_Success(t *testing.T) {
	compressor := &snappyCompressor{}

	tests := []struct {
		name string
		src  []byte
	}{
		{
			name: "empty input",
			src:  []byte{},
		},
		{
			name: "simple text",
			src:  []byte("hello world"),
		},
		{
			name: "repeated pattern",
			src:  bytes.Repeat([]byte("test"), 100),
		},
		{
			name: "binary data",
			src:  []byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xfe, 0xfd, 0xfc},
		},
		{
			name: "unicode text",
			src:  []byte("Hello 世界 🌍"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First compress the data
			compressed := snappy.Encode(nil, tt.src)

			// Create buffer for decompression
			buf := make([]byte, len(tt.src))

			// Test decompression
			err := compressor.Decompress(buf, compressed)
			require.NoError(t, err)

			// Verify the decompressed data matches original
			assert.Equal(t, tt.src, buf)
		})
	}
}

func TestSnappyCompressor_Decompress_BufferSizeMismatch(t *testing.T) {
	compressor := &snappyCompressor{}

	// Compress some test data
	originalData := []byte("test data for buffer size mismatch")
	compressed := snappy.Encode(nil, originalData)

	// Test with wrong buffer size (too small)
	smallBuf := make([]byte, len(originalData)-5)
	err := compressor.Decompress(smallBuf, compressed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compressed data mismatch")

	// Test with wrong buffer size (too large)
	largeBuf := make([]byte, len(originalData)+5)
	err = compressor.Decompress(largeBuf, compressed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compressed data mismatch")
}

func TestSnappyCompressor_DecompressedLen(t *testing.T) {
	compressor := &snappyCompressor{}

	tests := []struct {
		name string
		src  []byte
	}{
		{
			name: "empty input",
			src:  []byte{},
		},
		{
			name: "simple text",
			src:  []byte("hello world"),
		},
		{
			name: "repeated pattern",
			src:  bytes.Repeat([]byte("test"), 50),
		},
		{
			name: "large data",
			src:  bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First compress the data
			compressed := snappy.Encode(nil, tt.src)

			// Test DecompressedLen
			length, err := compressor.DecompressedLen(compressed)
			require.NoError(t, err)

			// Verify the length matches the original data length
			assert.Equal(t, len(tt.src), length)
		})
	}
}

func TestSnappyCompressor_RoundTrip(t *testing.T) {
	compressor := &snappyCompressor{}

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "empty data",
			data: []byte{},
		},
		{
			name: "single byte",
			data: []byte{0x42},
		},
		{
			name: "text data",
			data: []byte("Hello, World! This is a test string for compression."),
		},
		{
			name: "binary data",
			data: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0xff, 0xfe, 0xfd, 0xfc},
		},
		{
			name: "highly compressible",
			data: bytes.Repeat([]byte("A"), 1000),
		},
		{
			name: "mixed content",
			data: []byte("Mix of text and binary: \x00\x01\x02 Hello \xff\xfe World!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compress
			dst := make([]byte, 0, len(tt.data)*2) // Give enough space
			compressed := compressor.Compress(dst, tt.data)

			// Get decompressed length
			decompressedLen, err := compressor.DecompressedLen(compressed)
			require.NoError(t, err)
			assert.Equal(t, len(tt.data), decompressedLen)

			// Decompress
			decompressBuf := make([]byte, decompressedLen)
			err = compressor.Decompress(decompressBuf, compressed)
			require.NoError(t, err)

			// Verify round-trip worked
			assert.Equal(t, tt.data, decompressBuf)
		})
	}
}

func TestSnappyCompressor_LargeData(t *testing.T) {
	compressor := &snappyCompressor{}

	// Test with large data
	largeData := bytes.Repeat([]byte("This is a test string for large data compression. "), 10000)

	// Compress
	compressed := compressor.Compress(nil, largeData)

	// Verify compression ratio is reasonable for repetitive data
	compressionRatio := float64(len(compressed)) / float64(len(largeData))
	assert.Less(t, compressionRatio, 0.1, "Expected good compression ratio for repetitive data")

	// Verify round-trip
	decompressedLen, err := compressor.DecompressedLen(compressed)
	require.NoError(t, err)
	assert.Equal(t, len(largeData), decompressedLen)

	decompressBuf := make([]byte, decompressedLen)
	err = compressor.Decompress(decompressBuf, compressed)
	require.NoError(t, err)
	assert.Equal(t, largeData, decompressBuf)
}
