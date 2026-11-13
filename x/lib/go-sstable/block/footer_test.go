package block

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLayoutReader is a mock implementation of storage.ILayoutReader
type MockLayoutReader struct {
	data []byte
	size uint64
}

func NewMockLayoutReader(data []byte) *MockLayoutReader {
	return &MockLayoutReader{
		data: data,
		size: uint64(len(data)),
	}
}

func (m *MockLayoutReader) ReadAt(p []byte, off uint64) error {
	if off > m.size || off+uint64(len(p)) > m.size {
		return fmt.Errorf("read past end of file")
	}
	copy(p, m.data[off:off+uint64(len(p))])
	return nil
}

func (m *MockLayoutReader) Close() error {
	return nil
}

func TestFooterSerialization(t *testing.T) {
	// Create a footer with known values
	metaIndexBH := common.BlockHandle{
		Offset: 1234,
		Length: 5678,
	}
	footer := &Footer{
		Version:     common.TableV1,
		MetaIndexBH: metaIndexBH,
	}

	// Serialize the footer
	serialized := footer.Serialise()

	// Check if the footer size matches the expected size for TableV1
	assert.Equal(t, common.TableFooterSize[common.TableV1], len(serialized))

	// Check if the footer contains the correct magic number at the end
	magicNumberPos := len(serialized) - common.MagicNumberLen
	assert.Equal(t, common.MagicNumber, string(serialized[magicNumberPos:]))

	// Check if the footer contains the correct version before the magic number
	versionPos := magicNumberPos - common.TableVersionLen
	version := binary.LittleEndian.Uint32(serialized[versionPos:magicNumberPos])
	assert.Equal(t, uint32(common.TableV1), version)

	// Create a buffer to decode the BlockHandle
	metaIndexBuf := serialized[:versionPos]
	decodedBH := &common.BlockHandle{}
	n := decodedBH.DecodeFrom(metaIndexBuf)
	assert.Greater(t, n, 0)
	assert.Equal(t, metaIndexBH.Offset, decodedBH.Offset)
	assert.Equal(t, metaIndexBH.Length, decodedBH.Length)
}

func TestReadFooter(t *testing.T) {
	tests := []struct {
		name        string
		setupFooter func() ([]byte, *Footer)
		fileSize    uint64
		wantErr     bool
	}{
		{
			name: "Valid footer",
			setupFooter: func() ([]byte, *Footer) {
				metaIndexBH := common.BlockHandle{
					Offset: 1234,
					Length: 5678,
				}
				footer := &Footer{
					Version:     common.TableV1,
					MetaIndexBH: metaIndexBH,
				}
				return footer.Serialise(), footer
			},
			fileSize: uint64(common.TableFooterSize[common.TableV1]),
			wantErr:  false,
		},
		{
			name: "Invalid magic number",
			setupFooter: func() ([]byte, *Footer) {
				// Create a valid footer first
				metaIndexBH := common.BlockHandle{
					Offset: 1234,
					Length: 5678,
				}
				footer := &Footer{
					Version:     common.TableV1,
					MetaIndexBH: metaIndexBH,
				}
				data := footer.Serialise()

				// Corrupt the magic number
				copy(data[len(data)-common.MagicNumberLen:], "BADMAGIC")
				return data, nil
			},
			fileSize: uint64(common.TableFooterSize[common.TableV1]),
			wantErr:  true,
		},
		{
			name: "File too small",
			setupFooter: func() ([]byte, *Footer) {
				// Create a valid footer first
				metaIndexBH := common.BlockHandle{
					Offset: 1234,
					Length: 5678,
				}
				footer := &Footer{
					Version:     common.TableV1,
					MetaIndexBH: metaIndexBH,
				}
				return footer.Serialise()[:common.TableFooterSize[common.TableV1]-5], nil
			},
			fileSize: uint64(common.TableFooterSize[common.TableV1] - 5),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the test data
			fileData, expectedFooter := tt.setupFooter()
			reader := NewMockLayoutReader(fileData)

			// Call the function under test
			gotFooter, err := ReadFooter(reader, tt.fileSize)

			// Check if the error matches expectation
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			// If no error expected, check if the footer was read correctly
			require.NoError(t, err)
			assert.Equal(t, expectedFooter.Version, gotFooter.Version)
			assert.Equal(t, expectedFooter.MetaIndexBH.Offset, gotFooter.MetaIndexBH.Offset)
			assert.Equal(t, expectedFooter.MetaIndexBH.Length, gotFooter.MetaIndexBH.Length)
		})
	}
}

func TestReadFooterRoundTrip(t *testing.T) {
	// Create a footer with test values
	metaIndexBH := common.BlockHandle{
		Offset: 9876,
		Length: 5432,
	}
	originalFooter := &Footer{
		Version:     common.TableV1,
		MetaIndexBH: metaIndexBH,
	}

	// Serialize the footer
	serialized := originalFooter.Serialise()

	// Create a mock reader containing the serialized footer
	reader := NewMockLayoutReader(serialized)

	// Read the footer using ReadFooter
	readFooter, err := ReadFooter(reader, uint64(len(serialized)))

	// Check that no error occurred
	require.NoError(t, err)

	// Check that the read footer matches the original
	assert.Equal(t, originalFooter.Version, readFooter.Version)
	assert.Equal(t, originalFooter.MetaIndexBH.Offset, readFooter.MetaIndexBH.Offset)
	assert.Equal(t, originalFooter.MetaIndexBH.Length, readFooter.MetaIndexBH.Length)

	// Serialize the read footer and check it matches the original serialized data
	reserialized := readFooter.Serialise()
	assert.Equal(t, serialized, reserialized)
}

func TestFooterWithLargeOffsets(t *testing.T) {
	// Create a footer with large offset values
	metaIndexBH := common.BlockHandle{
		Offset: 0xFFFFFFFF, // Large 32-bit value
		Length: 0xFFFFFFFF, // Large 32-bit value
	}
	originalFooter := &Footer{
		Version:     common.TableV1,
		MetaIndexBH: metaIndexBH,
	}

	// Serialize and verify round-trip
	serialized := originalFooter.Serialise()
	reader := NewMockLayoutReader(serialized)
	readFooter, err := ReadFooter(reader, uint64(len(serialized)))

	require.NoError(t, err)
	assert.Equal(t, metaIndexBH.Offset, readFooter.MetaIndexBH.Offset)
	assert.Equal(t, metaIndexBH.Length, readFooter.MetaIndexBH.Length)
}

// Test for ReadFooter when file is smaller than MaxPossibleFooterSize
func TestReadFooterSmallFile(t *testing.T) {
	// Create a valid but small footer
	metaIndexBH := common.BlockHandle{
		Offset: 10,
		Length: 20,
	}
	originalFooter := &Footer{
		Version:     common.TableV1,
		MetaIndexBH: metaIndexBH,
	}

	serialized := originalFooter.Serialise()

	// Create a mock reader with just the serialized footer
	reader := NewMockLayoutReader(serialized)

	// Call ReadFooter with file size equal to the serialized footer size
	readFooter, err := ReadFooter(reader, uint64(len(serialized)))

	require.NoError(t, err)
	assert.Equal(t, originalFooter.Version, readFooter.Version)
	assert.Equal(t, originalFooter.MetaIndexBH.Offset, readFooter.MetaIndexBH.Offset)
	assert.Equal(t, originalFooter.MetaIndexBH.Length, readFooter.MetaIndexBH.Length)
}
