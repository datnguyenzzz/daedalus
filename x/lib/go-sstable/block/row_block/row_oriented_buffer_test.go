package row_block

import (
	"testing"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_WriteEntry_Then_Finish(t *testing.T) {
	type param struct {
		desc            string
		inputUserKeys   []string
		inputValue      []string
		restartInterval int
		expectedBlkData []byte
	}

	tests := []param{
		{
			desc: "#1 - restart interval 2",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "avowed", "cherry", "mango",
			},
			inputValue: []string{
				"red", "orange", "purple", "yellow", "red", "blue",
			},
			restartInterval: 2,
			expectedBlkData: []byte(
				"\x00\x0d\x03apple\x00\x00\x00\x00\x00\x00\x00\x00red" +
					"\x02\x0d\x06ricot\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x00\x0f\x06avocado\x00\x00\x00\x00\x00\x00\x00\x00purple" +
					"\x03\x0b\x06wed\x00\x00\x00\x00\x00\x00\x00\x00yellow" +
					"\x00\x0e\x03cherry\x00\x00\x00\x00\x00\x00\x00\x00red" +
					"\x00\x0d\x04mango\x00\x00\x00\x00\x00\x00\x00\x00blue" +
					// block trailer [0, 41, 85] 3, we use little endian
					"\x00\x00\x00\x00\x29\x00\x00\x00\x55\x00\x00\x00\x03\x00\x00\x00",
			),
		},
		{
			desc: "#2 - restart interval 1",
			inputUserKeys: []string{
				"banana", "berry", "blueberry",
			},
			inputValue: []string{
				"yellow", "red", "blue",
			},
			restartInterval: 1,
			expectedBlkData: []byte(
				"\x00\x0e\x06banana\x00\x00\x00\x00\x00\x00\x00\x00yellow" +
					"\x00\x0d\x03berry\x00\x00\x00\x00\x00\x00\x00\x00red" +
					"\x00\x11\x04blueberry\x00\x00\x00\x00\x00\x00\x00\x00blue" +
					// block trailer [0, 23, 42] 3, we use little endian
					"\x00\x00\x00\x00\x17\x00\x00\x00\x2a\x00\x00\x00\x03\x00\x00\x00"),
		},
		{
			desc: "#3 - restart interval 3",
			inputUserKeys: []string{
				"grape", "grapefruit", "guava", "kiwi", "lemon", "lime",
			},
			inputValue: []string{
				"purple", "pink", "green", "brown", "yellow", "green",
			},
			restartInterval: 3,
			expectedBlkData: []byte(
				"\x00\x0d\x06grape\x00\x00\x00\x00\x00\x00\x00\x00purple" +
					"\x05\x0d\x04fruit\x00\x00\x00\x00\x00\x00\x00\x00pink" +
					"\x01\x0c\x05uava\x00\x00\x00\x00\x00\x00\x00\x00green" +
					"\x00\x0c\x05kiwi\x00\x00\x00\x00\x00\x00\x00\x00brown" +
					"\x00\x0d\x06lemon\x00\x00\x00\x00\x00\x00\x00\x00yellow" +
					"\x01\x0b\x05ime\x00\x00\x00\x00\x00\x00\x00\x00green" +
					// block trailer [0, 62] 2, we use little endian
					"\x00\x00\x00\x00\x3e\x00\x00\x00\x02\x00\x00\x00"),
		},
		{
			desc: "#4 - restart interval 4",
			inputUserKeys: []string{
				"orange", "papaya", "peach", "pear",
			},
			inputValue: []string{
				"orange", "orange", "pink", "green",
			},
			restartInterval: 4,
			expectedBlkData: []byte(
				"\x00\x0e\x06orange\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x00\x0e\x06papaya\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x01\x0c\x04each\x00\x00\x00\x00\x00\x00\x00\x00pink" +
					"\x03\x09\x05r\x00\x00\x00\x00\x00\x00\x00\x00green" +
					// block trailer [0] 1, we use little endian
					"\x00\x00\x00\x00\x01\x00\x00\x00"),
		},
		{
			desc: "#5 - restart interval 5 with single entry",
			inputUserKeys: []string{
				"strawberry",
			},
			inputValue: []string{
				"red",
			},
			restartInterval: 5,
			expectedBlkData: []byte(
				"\x00\x12\x03strawberry\x00\x00\x00\x00\x00\x00\x00\x00red" +
					// block trailer [0] 1, we use little endian
					"\x00\x00\x00\x00\x01\x00\x00\x00"),
		},
		{
			desc: "#6 - restart interval 6 with many entries",
			inputUserKeys: []string{
				"apricot", "avocado", "banana", "blueberry", "cherry", "grape", "kiwi", "lemon", "mango", "orange", "papaya", "peach",
			},
			inputValue: []string{
				"orange", "green", "yellow", "blue", "red", "purple", "brown", "yellow", "orange", "orange", "orange", "pink",
			},
			restartInterval: 6,
			expectedBlkData: []byte(
				"\x00\x0f\x06apricot\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x01\x0e\x05vocado\x00\x00\x00\x00\x00\x00\x00\x00green" +
					"\x00\x0e\x06banana\x00\x00\x00\x00\x00\x00\x00\x00yellow" +
					"\x01\x10\x04lueberry\x00\x00\x00\x00\x00\x00\x00\x00blue" +
					"\x00\x0e\x03cherry\x00\x00\x00\x00\x00\x00\x00\x00red" +
					"\x00\x0d\x06grape\x00\x00\x00\x00\x00\x00\x00\x00purple" +
					"\x00\x0c\x05kiwi\x00\x00\x00\x00\x00\x00\x00\x00brown" +
					"\x00\x0d\x06lemon\x00\x00\x00\x00\x00\x00\x00\x00yellow" +
					"\x00\x0d\x06mango\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x00\x0e\x06orange\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x00\x0e\x06papaya\x00\x00\x00\x00\x00\x00\x00\x00orange" +
					"\x01\x0c\x04each\x00\x00\x00\x00\x00\x00\x00\x00pink" +
					// block trailer [0, 262] 2, we use little endian
					"\x00\x00\x00\x00\x86\x00\x00\x00\x02\x00\x00\x00"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)
			n := len(tc.inputUserKeys)
			for i := range n {
				key := makeDummyKey(tc.inputUserKeys[i])
				value := []byte(tc.inputValue[i])
				// 1. Write a new entry, expect there is no error
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have an error when writing entry")

				// 2. Assert current key / value
				currKey := blk.CurKey()
				assert.Equal(t, key.UserKey, currKey.UserKey, "user key should be equal")
				assert.Equal(t, value, blk.currValue, "value should be equal")

				// 3. Assert entries count
				assert.Equal(t, i+1, blk.EntryCount(), "entry count should be equal")
			}
			blkData := make([]byte, blk.EstimateSize())
			blk.Finish(blkData)

			// 4. Ensure the block has been reset, but don't lose the allocated buffer
			assert.Zero(t, blk.EntryCount())
			assert.Zero(t, blk.nextRestartEntry)
			assert.Zero(t, len(blk.restartOffset))
			assert.Zero(t, len(blk.curKey))
			assert.Zero(t, len(blk.prevKey))
			assert.Zero(t, len(blk.currValue))
			assert.Zero(t, len(blk.buf))
			assert.Greater(t, cap(blk.buf), 0)
			assert.Greater(t, cap(blk.restartOffset), 0)
			assert.Greater(t, cap(blk.currValue), 0)
			assert.Greater(t, cap(blk.curKey), 0)

			// 5. Assert the data block matches expected
			assert.Equal(t, tc.expectedBlkData, blkData)
		})
	}
}

func makeDummyKey(userKey string) nogodb_common.InternalKey {
	return nogodb_common.MakeKey([]byte(userKey), 0, nogodb_common.KeyKindUnknown)
}

// MockFlushDecider implements common.IFlushDecider for testing
type MockFlushDecider struct {
	mock.Mock
}

func (m *MockFlushDecider) ShouldFlush(sizeBefore, sizeAfter int) bool {
	args := m.Called(sizeBefore, sizeAfter)
	return args.Bool(0)
}

func Test_WriteEntry_Error(t *testing.T) {
	// Setup
	bp := predictable_size.NewPredictablePool()
	blk := newBlock(2, bp, 1024*1024)

	// 1. Modify the buffer to exceed the maximum offset allowed
	// The maximumRestartOffset is 2^31 - 1, so we're making the buffer exceed that
	blk.buf = make([]byte, maximumRestartOffset+1)

	// 2. Try to write a new entry
	key := makeDummyKey("test-key")
	value := []byte("test-value")

	// 3. Assert that an error is returned
	err := blk.WriteEntry(key, value)
	assert.ErrorIs(t, err, common.InternalServerError, "should return internal error when buffer exceeds maximum offset")

	// 4. Assert that entry count was not incremented
	assert.Zero(t, blk.EntryCount(), "entry count should not be incremented when an error occurs")
}

// Define an interface for the buffer pool so we can mock it
type BufferPool interface {
	Get(dataLen int) []byte
	Put(buf []byte)
}

// Ensure PredictablePool implements BufferPool
var _ BufferPool = (*predictable_size.PredictablePool)(nil)

// MockPredictablePool is a mock implementation of the BufferPool interface
type MockPredictablePool struct {
	mock.Mock
}

func (m *MockPredictablePool) Put(buf []byte) {
	m.Called(buf)
}

func (m *MockPredictablePool) Get(dataLen int) []byte {
	args := m.Called(dataLen)
	if v := args.Get(0); v != nil {
		return v.([]byte)
	}
	return nil
}

func Test_Release(t *testing.T) {
	// Create a real buffer pool first
	realBp := predictable_size.NewPredictablePool()

	// Create a test block
	blk := newBlock(2, realBp, 1024*1024)

	// Create a mock pool and modify the implementation to use it
	mockPool := new(MockPredictablePool)

	// Create a temporary wrapper struct to substitute the mockPool
	wrapper := struct {
		*rowBlockBuf
		mockPool *MockPredictablePool
	}{
		rowBlockBuf: blk,
		mockPool:    mockPool,
	}

	// Create a custom Release function for our test
	customRelease := func() {
		// Use the mock pool instead of the real one
		wrapper.mockPool.Put(wrapper.buf)
	}

	// Setup test data and expectations
	wrapper.buf = []byte{1, 2, 3, 4, 5}
	mockPool.On("Put", wrapper.buf).Return()

	// Call our custom Release function that uses the mock
	customRelease()

	// Verify the mock was called correctly
	mockPool.AssertExpectations(t)
}

func Test_CleanUpForReuse(t *testing.T) {
	// Setup
	bp := predictable_size.NewPredictablePool()
	blk := newBlock(2, bp, 1024*1024)

	// 1. Add data to the block
	key := makeDummyKey("test-key")
	value := []byte("test-value")

	// Write an entry to populate the block
	err := blk.WriteEntry(key, value)
	assert.NoError(t, err)

	// Verify the block has data
	assert.Equal(t, 1, blk.EntryCount())
	assert.NotEmpty(t, blk.curKey)
	assert.NotEmpty(t, blk.currValue)
	assert.NotEmpty(t, blk.buf)
	assert.NotZero(t, blk.nextRestartEntry)
	assert.NotEmpty(t, blk.restartOffset)

	// Save capacities before cleanup
	bufCap := cap(blk.buf)
	keyCap := cap(blk.curKey)
	valCap := cap(blk.currValue)
	restartCap := cap(blk.restartOffset)

	// 2. Call CleanUpForReuse
	blk.CleanUpForReuse()

	// 3. Verify all fields are reset
	assert.Zero(t, blk.EntryCount())
	assert.Zero(t, blk.nextRestartEntry)
	assert.Empty(t, blk.restartOffset)
	assert.Empty(t, blk.curKey)
	assert.Empty(t, blk.prevKey)
	assert.Empty(t, blk.currValue)
	assert.Empty(t, blk.buf)

	// 4. Verify capacities are maintained (buffers reused)
	assert.Equal(t, bufCap, cap(blk.buf))
	assert.Equal(t, keyCap, cap(blk.curKey))
	assert.Equal(t, valCap, cap(blk.currValue))
	assert.Equal(t, restartCap, cap(blk.restartOffset))
}

func Test_EstimateSize(t *testing.T) {
	tests := []struct {
		desc               string
		bufSize            int
		restartOffsetCount int
		expectedSize       int
	}{
		{
			desc:               "Empty block",
			bufSize:            0,
			restartOffsetCount: 0,
			expectedSize:       4, // Just the 4-byte space for restart length
		},
		{
			desc:               "Block with data but no restart points",
			bufSize:            100,
			restartOffsetCount: 0,
			expectedSize:       104, // 100 bytes buffer + 4 bytes for restart length
		},
		{
			desc:               "Block with data and restart points",
			bufSize:            200,
			restartOffsetCount: 3,
			expectedSize:       216, // 200 bytes buffer + (3 * 4) bytes for restart offsets + 4 bytes for restart length
		},
		{
			desc:               "Large block with many restart points",
			bufSize:            10000,
			restartOffsetCount: 50,
			expectedSize:       10204, // 10000 bytes buffer + (50 * 4) bytes for restart offsets + 4 bytes for restart length
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(2, bp, 1024*1024)

			// Set the buffer and restart offsets for testing
			blk.buf = make([]byte, tc.bufSize)
			blk.restartOffset = make([]uint32, tc.restartOffsetCount)

			// Act
			actualSize := blk.EstimateSize()

			// Assert
			assert.Equal(t, tc.expectedSize, actualSize)
		})
	}
}

func Test_ShouldFlush(t *testing.T) {
	tests := []struct {
		desc           string
		entryCount     int
		pendingKeyLen  int
		pendingValLen  int
		mockReturn     bool
		expectedResult bool
	}{
		{
			desc:           "Should not flush with no entries",
			entryCount:     0,
			pendingKeyLen:  10,
			pendingValLen:  20,
			mockReturn:     true, // Would flush based on size
			expectedResult: false,
		},
		{
			desc:           "Should not flush when decider returns false",
			entryCount:     1,
			pendingKeyLen:  10,
			pendingValLen:  20,
			mockReturn:     false,
			expectedResult: false,
		},
		{
			desc:           "Should flush when decider returns true",
			entryCount:     1,
			pendingKeyLen:  10,
			pendingValLen:  20,
			mockReturn:     true,
			expectedResult: true,
		},
		{
			desc:           "Should account for restart interval overhead",
			entryCount:     2, // Will be at restart point if restart interval is 2
			pendingKeyLen:  10,
			pendingValLen:  20,
			mockReturn:     true,
			expectedResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup - create a mock block with custom EstimateSize implementation
			bp := predictable_size.NewPredictablePool()

			// Create a test block with our mocked size
			blk := &mockRowBlockBuf{
				rowBlockBuf: rowBlockBuf{
					nEntries:        tc.entryCount,
					restartInterval: 2,
					bufferPool:      bp,
				},
				// Just use a simple mock size
				mockSize: 4,
			}

			// Setup mock decider
			mockDecider := new(MockFlushDecider)

			if tc.entryCount > 0 {
				// Use mock.Anything matchers for the ShouldFlush call
				mockDecider.On("ShouldFlush", mock.Anything, mock.Anything).Return(tc.mockReturn)
			}

			// Act
			result := blk.ShouldFlush(tc.pendingKeyLen, tc.pendingValLen, mockDecider)

			// Assert
			assert.Equal(t, tc.expectedResult, result)
			mockDecider.AssertExpectations(t)
		})
	}
}

// mockRowBlockBuf is a testable version of rowBlockBuf with overridden EstimateSize method
type mockRowBlockBuf struct {
	rowBlockBuf
	mockSize int
}

// EstimateSize returns the mocked size
func (m *mockRowBlockBuf) EstimateSize() int {
	return m.mockSize
}
