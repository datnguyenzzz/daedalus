package common

import (
	"testing"

	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFetcher is a mock implementation of Fetcher interface for testing
type MockFetcher struct {
	data     []byte
	released bool
}

func NewMockFetcher(data []byte) *MockFetcher {
	return &MockFetcher{
		data:     data,
		released: false,
	}
}

func (m *MockFetcher) Load() []byte {
	return m.data
}

func (m *MockFetcher) Release() {
	m.released = true
}

func (m *MockFetcher) IsReleased() bool {
	return m.released
}

func TestValueSource_Constants(t *testing.T) {
	// Test that ValueSource constants have expected values
	assert.Equal(t, ValueSource(0), ValueFromUnknown)
	assert.Equal(t, ValueSource(1), ValueFromBuffer)
	assert.Equal(t, ValueSource(2), ValueFromCache)
}

func TestBufferPoolFetcher_Reserve(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	fetcher := &BufferPoolFetcher{pool: pool}

	size := 100
	fetcher.Reserve(size)

	// Check that buffer is allocated with correct size
	assert.NotNil(t, fetcher.val)
	assert.Equal(t, size, len(fetcher.val))

	// Clean up
	fetcher.Release()
}

func TestBufferPoolFetcher_Set(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	fetcher := &BufferPoolFetcher{pool: pool}

	// test case - with reservation
	testData := []byte("test data with reservation")
	fetcher.Reserve(len(testData))
	fetcher.Set(testData)
	assert.Equal(t, testData, fetcher.val)

	// test case - Without reservation
	testData = []byte("test data Without reservation")
	fetcher.Set(testData)
	assert.Equal(t, testData, fetcher.val)

	// Clean up
	fetcher.Release()
}

func TestBufferPoolFetcher_Load(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	fetcher := &BufferPoolFetcher{pool: pool}

	// test case - with reservation
	testData := []byte("test data with reservation")
	fetcher.Reserve(len(testData))
	fetcher.Set(testData)
	assert.Equal(t, testData, fetcher.Load())

	// test case - Without reservation
	testData = []byte("test data Without reservation")
	fetcher.Set(testData)
	assert.Equal(t, testData, fetcher.Load())

	// Clean up
	fetcher.Release()
}

func TestBufferPoolFetcher_Release(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	fetcher := &BufferPoolFetcher{pool: pool}

	fetcher.Reserve(100)
	assert.NotNil(t, fetcher.val)

	fetcher.Release()
	assert.Nil(t, fetcher.val)

	// Test multiple releases don't cause panic
	fetcher.Release()
	assert.Nil(t, fetcher.val)
}

func TestInternalLazyValue_ValueFromBuffer(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	iv := &InternalLazyValue{}

	testData := []byte("test buffer data")
	iv.ReserveBuffer(pool, len(testData))

	// Copy test data into reserved buffer
	copy(iv.BufferFetcher.Load(), testData)

	// Test Value() method
	value := iv.Value()
	assert.Equal(t, testData, value)
	assert.Equal(t, ValueFromBuffer, iv.ValueSource)

	// Clean up
	iv.Release()
}

func TestInternalLazyValue_ValueFromCache(t *testing.T) {
	testData := []byte("test cache data")
	mockFetcher := NewMockFetcher(testData)

	iv := &InternalLazyValue{
		ValueSource:  ValueFromCache,
		CacheFetcher: mockFetcher,
	}

	// Test Value() method
	value := iv.Value()
	assert.Equal(t, testData, value)
	assert.Equal(t, ValueFromCache, iv.ValueSource)

	// Test Release() method
	assert.False(t, mockFetcher.IsReleased())
	iv.Release()
	assert.True(t, mockFetcher.IsReleased())
}

func TestInternalLazyValue_ValueFromUnknown_Panic(t *testing.T) {
	iv := &InternalLazyValue{
		ValueSource: ValueFromUnknown,
	}

	// Test that Value() panics for unknown source
	assert.Panics(t, func() {
		iv.Value()
	})

	// Test that Release() panics for unknown source
	assert.Panics(t, func() {
		iv.Release()
	})
}

func TestInternalLazyValue_ReserveBuffer(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	iv := &InternalLazyValue{}

	size := 256
	iv.ReserveBuffer(pool, size)

	// Check that buffer is properly set up
	assert.Equal(t, ValueFromBuffer, iv.ValueSource)
	assert.NotNil(t, iv.BufferFetcher)
	assert.Equal(t, size, len(iv.BufferFetcher.Load()))
	assert.Equal(t, pool, iv.BufferFetcher.pool)

	// Clean up
	iv.Release()
}

func TestInternalLazyValue_SetBufferValue_Success(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	iv := &InternalLazyValue{}

	testData := []byte("test data")
	iv.ReserveBuffer(pool, len(testData))

	err := iv.SetBufferValue(testData)
	require.NoError(t, err)

	// Verify the value was set correctly
	value := iv.Value()
	assert.Equal(t, testData, value)

	// Clean up
	iv.Release()
}

func TestInternalLazyValue_SetBufferValue_WrongSource(t *testing.T) {
	iv := &InternalLazyValue{
		ValueSource: ValueFromCache,
	}

	testData := []byte("test data")
	err := iv.SetBufferValue(testData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value source")
	assert.Contains(t, err.Error(), "has not supported this function")
}

func TestInternalLazyValue_SetBufferValue_WrongSize(t *testing.T) {
	pool := predictable_size.NewPredictablePool()
	iv := &InternalLazyValue{}

	// Reserve buffer of size 10
	iv.ReserveBuffer(pool, 10)

	// Try to set data with different size
	testData := []byte("this is longer than 10 bytes")
	err := iv.SetBufferValue(testData)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "the buffered capacity is different")

	// Clean up
	iv.Release()
}

func TestInternalLazyValue_SetCacheFetcher_Success(t *testing.T) {
	testData := []byte("cache test data")
	mockFetcher := NewMockFetcher(testData)

	iv := &InternalLazyValue{
		ValueSource: ValueFromCache,
	}

	err := iv.SetCacheFetcher(mockFetcher)
	require.NoError(t, err)

	// Verify the fetcher was set correctly
	assert.Equal(t, mockFetcher, iv.CacheFetcher)
	value := iv.Value()
	assert.Equal(t, testData, value)
}

func TestInternalLazyValue_SetCacheFetcher_WrongSource(t *testing.T) {
	testData := []byte("test data")
	mockFetcher := NewMockFetcher(testData)

	iv := &InternalLazyValue{
		ValueSource: ValueFromBuffer,
	}

	err := iv.SetCacheFetcher(mockFetcher)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value source")
	assert.Contains(t, err.Error(), "has not supported this function")
}

func TestInternalKV_Structure(t *testing.T) {
	// Create an InternalKey for testing
	userKey := []byte("test-key")
	seqNum := uint64(123)
	keyType := KeyKindSet
	internalKey := MakeKey(userKey, SeqNum(seqNum), keyType)

	// Create an InternalLazyValue for testing
	testData := []byte("test value data")
	mockFetcher := NewMockFetcher(testData)
	internalValue := InternalLazyValue{
		ValueSource:  ValueFromCache,
		CacheFetcher: mockFetcher,
	}

	// Create InternalKV
	kv := InternalKV{
		K: internalKey,
		V: internalValue,
	}

	// Test that the struct holds the correct values
	assert.Equal(t, internalKey, kv.K)
	assert.Equal(t, internalValue, kv.V)

	// Test that we can access the value through the InternalKV
	value := kv.V.Value()
	assert.Equal(t, testData, value)
}

func TestInternalLazyValue_MultipleOperations(t *testing.T) {
	pool := predictable_size.NewPredictablePool()

	tests := []struct {
		name        string
		setupValue  func() *InternalLazyValue
		testData    []byte
		expectError bool
	}{
		{
			name: "Buffer operations",
			setupValue: func() *InternalLazyValue {
				iv := &InternalLazyValue{}
				iv.ReserveBuffer(pool, 19)
				return iv
			},
			testData:    []byte("buffer test data123"),
			expectError: false,
		},
		{
			name: "Cache operations",
			setupValue: func() *InternalLazyValue {
				return &InternalLazyValue{
					ValueSource: ValueFromCache,
				}
			},
			testData:    []byte("cache test data"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := tt.setupValue()

			switch iv.ValueSource {
			case ValueFromBuffer:
				err := iv.SetBufferValue(tt.testData)
				if tt.expectError {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)

				value := iv.Value()
				assert.Equal(t, tt.testData, value)
				iv.Release()

			case ValueFromCache:
				mockFetcher := NewMockFetcher(tt.testData)
				err := iv.SetCacheFetcher(mockFetcher)
				if tt.expectError {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)

				value := iv.Value()
				assert.Equal(t, tt.testData, value)

				assert.False(t, mockFetcher.IsReleased())
				iv.Release()
				assert.True(t, mockFetcher.IsReleased())
			}
		})
	}
}

func TestFetcher_Interface(t *testing.T) {
	// Verify that MockFetcher implements Fetcher interface
	var _ Fetcher = (*MockFetcher)(nil)
	var _ Fetcher = NewMockFetcher([]byte("test"))

	// Verify that BufferPoolFetcher implements Fetcher interface
	var _ Fetcher = (*BufferPoolFetcher)(nil)
}
