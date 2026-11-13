package row_block

import (
	"testing"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	"github.com/stretchr/testify/assert"
)

func TestDataBlockIterator_readEntry(t *testing.T) {
	type testEntry struct {
		offset        uint64
		expectedKey   string
		expectedValue string
	}

	type testCase struct {
		desc            string
		inputUserKeys   []string
		inputValues     []string
		restartInterval int
		testEntries     []testEntry
	}

	tests := []testCase{
		{
			desc: "#1 - restart interval 2, test multiple entries",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "avowed", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "purple", "yellow", "red", "blue",
			},
			restartInterval: 2,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "apple", expectedValue: "red"},
				{offset: 19, expectedKey: "ricot", expectedValue: "orange"},
				{offset: 41, expectedKey: "avocado", expectedValue: "purple"},
			},
		},
		{
			desc: "#2 - restart interval 1, all entries are restart points",
			inputUserKeys: []string{
				"banana", "berry", "blueberry",
			},
			inputValues: []string{
				"yellow", "red", "blue",
			},
			restartInterval: 1,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "banana", expectedValue: "yellow"},
				{offset: 23, expectedKey: "berry", expectedValue: "red"},
				{offset: 42, expectedKey: "blueberry", expectedValue: "blue"},
			},
		},
		{
			desc: "#3 - restart interval 3, mixed restart and compressed entries",
			inputUserKeys: []string{
				"grape", "grapefruit", "guava", "kiwi", "lemon", "lime",
			},
			inputValues: []string{
				"purple", "pink", "green", "brown", "yellow", "green",
			},
			restartInterval: 3,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "grape", expectedValue: "purple"},
				{offset: 22, expectedKey: "fruit", expectedValue: "pink"},
				{offset: 42, expectedKey: "uava", expectedValue: "green"},
				{offset: 62, expectedKey: "kiwi", expectedValue: "brown"},
			},
		},
		{
			desc: "#4 - single entry",
			inputUserKeys: []string{
				"strawberry",
			},
			inputValues: []string{
				"red",
			},
			restartInterval: 5,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "strawberry", expectedValue: "red"},
			},
		},
		{
			desc: "#5 - large restart interval",
			inputUserKeys: []string{
				"orange", "papaya", "peach", "pear",
			},
			inputValues: []string{
				"orange", "orange", "pink", "green",
			},
			restartInterval: 4,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "orange", expectedValue: "orange"},
				{offset: 23, expectedKey: "papaya", expectedValue: "orange"},
				{offset: 46, expectedKey: "each", expectedValue: "pink"},
				{offset: 65, expectedKey: "r", expectedValue: "green"},
			},
		},
		{
			desc: "#6 - empty values",
			inputUserKeys: []string{
				"key1", "key2", "key3",
			},
			inputValues: []string{
				"", "", "",
			},
			restartInterval: 2,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "key1", expectedValue: ""},
				{offset: 15, expectedKey: "2", expectedValue: ""},
			},
		},
		{
			desc: "#7 - keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval: 3,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "prefix_key_001", expectedValue: "value1"},
				{offset: 31, expectedKey: "2", expectedValue: "value2"},
				{offset: 49, expectedKey: "3", expectedValue: "value3"},
				{offset: 67, expectedKey: "prefix_key_004", expectedValue: "value4"},
			},
		},
		{
			desc: "#8 - sequential read all entries",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "banana", "cherry", "grape",
			},
			inputValues: []string{
				"red", "orange", "green", "yellow", "red", "purple",
			},
			restartInterval: 2,
			testEntries: []testEntry{
				{offset: 0, expectedKey: "apple", expectedValue: "red"},
				{offset: 19, expectedKey: "ricot", expectedValue: "orange"},
				{offset: 41, expectedKey: "avocado", expectedValue: "green"},
				{offset: 64, expectedKey: "banana", expectedValue: "yellow"},
				{offset: 87, expectedKey: "cherry", expectedValue: "red"},
				{offset: 107, expectedKey: "grape", expectedValue: "purple"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data using the existing writer
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			// Write all entries to the block
			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			// Finalize the block
			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test readEntry on specific offsets
			cmp := nogodb_common.NewComparer()

			for _, entry := range tc.testEntries {
				lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
				lz.ReserveBuffer(bp, len(blockData))
				_ = lz.SetBufferValue(blockData)
				iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)
				iter.offset = entry.offset

				iter.readEntry()
				actualKey := nogodb_common.DeserializeKey(iter.key)
				assert.Equal(t, entry.expectedKey, string(actualKey.UserKey), "key should match at offset %d", entry.offset)
				assert.Equal(t, entry.expectedValue, string(iter.value), "value should match at offset %d", entry.offset)
				assert.Greater(t, iter.nextOffset, iter.offset, "nextOffset should be greater than current offset")
				assert.LessOrEqual(t, iter.nextOffset, iter.trailerOffset, "nextOffset should not exceed trailer offset")
			}
		})
	}
}

func TestDataBlockIterator_First(t *testing.T) {
	type testCase struct {
		desc               string
		inputUserKeys      []string
		inputValues        []string
		restartInterval    int
		expectedFirstKey   string
		expectedFirstValue string
	}

	tests := []testCase{
		{
			desc: "#1 - restart interval 2",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "avowed", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "purple", "yellow", "red", "blue",
			},
			restartInterval:    2,
			expectedFirstKey:   "apple",
			expectedFirstValue: "red",
		},
		{
			desc: "#2 - restart interval 1",
			inputUserKeys: []string{
				"banana", "berry", "blueberry",
			},
			inputValues: []string{
				"yellow", "red", "blue",
			},
			restartInterval:    1,
			expectedFirstKey:   "banana",
			expectedFirstValue: "yellow",
		},
		{
			desc: "#3 - restart interval 3",
			inputUserKeys: []string{
				"grape", "grapefruit", "guava", "kiwi", "lemon", "lime",
			},
			inputValues: []string{
				"purple", "pink", "green", "brown", "yellow", "green",
			},
			restartInterval:    3,
			expectedFirstKey:   "grape",
			expectedFirstValue: "purple",
		},
		{
			desc: "#4 - single entry",
			inputUserKeys: []string{
				"strawberry",
			},
			inputValues: []string{
				"red",
			},
			restartInterval:    5,
			expectedFirstKey:   "strawberry",
			expectedFirstValue: "red",
		},
		{
			desc: "#5 - large restart interval",
			inputUserKeys: []string{
				"orange", "papaya", "peach", "pear",
			},
			inputValues: []string{
				"orange", "orange", "pink", "green",
			},
			restartInterval:    4,
			expectedFirstKey:   "orange",
			expectedFirstValue: "orange",
		},
		{
			desc: "#6 - empty first value",
			inputUserKeys: []string{
				"key1", "key2", "key3",
			},
			inputValues: []string{
				"", "value2", "value3",
			},
			restartInterval:    2,
			expectedFirstKey:   "key1",
			expectedFirstValue: "",
		},
		{
			desc: "#7 - keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval:    3,
			expectedFirstKey:   "prefix_key_001",
			expectedFirstValue: "value1",
		},
		{
			desc: "#8 - very long first key and value",
			inputUserKeys: []string{
				"verylongkeyname1234567890abcdefghijklmnopqrstuvwxyz",
				"short",
				"medium_key",
			},
			inputValues: []string{
				"verylongvaluecontent1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
				"small",
				"medium_value",
			},
			restartInterval:    2,
			expectedFirstKey:   "verylongkeyname1234567890abcdefghijklmnopqrstuvwxyz",
			expectedFirstValue: "verylongvaluecontent1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data using the existing writer
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test First()
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			firstKV := iter.First()

			assert.NotNil(t, firstKV, "First() should return non-nil InternalKV")
			assert.Equal(t, tc.expectedFirstKey, string(firstKV.K.UserKey), "First() should return the first key")
			assert.Equal(t, tc.expectedFirstValue, string(firstKV.V.Value()), "First() should return the first value")
			assert.Equal(t, uint64(0), iter.offset, "iterator offset should be 0 after First()")
			assert.Greater(t, iter.nextOffset, uint64(0), "nextOffset should be greater than 0 after First()")
			assert.LessOrEqual(t, iter.nextOffset, iter.trailerOffset, "nextOffset should not exceed trailer offset")

			// Verify iterator's internal key and value are set
			actualIterKey := nogodb_common.DeserializeKey(iter.key)
			assert.Equal(t, tc.expectedFirstKey, string(actualIterKey.UserKey), "iterator's internal key should match first key")
			assert.Equal(t, tc.expectedFirstValue, string(iter.value), "iterator's internal value should match first value")
		})
	}
}

func TestBlockIterator_Next(t *testing.T) {
	type expectedEntry struct {
		expectedKey   string
		expectedValue string
	}

	type testCase struct {
		desc             string
		inputUserKeys    []string
		inputValues      []string
		restartInterval  int
		expectedSequence []expectedEntry
	}

	tests := []testCase{
		{
			desc: "#1 - restart interval 2, test sequential iteration",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "avowed", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "purple", "yellow", "red", "blue",
			},
			restartInterval: 2,
			expectedSequence: []expectedEntry{
				{expectedKey: "apple", expectedValue: "red"},
				{expectedKey: "apricot", expectedValue: "orange"},
				{expectedKey: "avocado", expectedValue: "purple"},
				{expectedKey: "avowed", expectedValue: "yellow"},
				{expectedKey: "cherry", expectedValue: "red"},
				{expectedKey: "mango", expectedValue: "blue"},
			},
		},
		{
			desc: "#2 - restart interval 1, all entries are restart points",
			inputUserKeys: []string{
				"banana", "berry", "blueberry",
			},
			inputValues: []string{
				"yellow", "red", "blue",
			},
			restartInterval: 1,
			expectedSequence: []expectedEntry{
				{expectedKey: "banana", expectedValue: "yellow"},
				{expectedKey: "berry", expectedValue: "red"},
				{expectedKey: "blueberry", expectedValue: "blue"},
			},
		},
		{
			desc: "#3 - restart interval 3, mixed restart and compressed entries",
			inputUserKeys: []string{
				"grape", "grapefruit", "guava", "kiwi", "lemon", "lime",
			},
			inputValues: []string{
				"purple", "pink", "green", "brown", "yellow", "green",
			},
			restartInterval: 3,
			expectedSequence: []expectedEntry{
				{expectedKey: "grape", expectedValue: "purple"},
				{expectedKey: "grapefruit", expectedValue: "pink"},
				{expectedKey: "guava", expectedValue: "green"},
				{expectedKey: "kiwi", expectedValue: "brown"},
				{expectedKey: "lemon", expectedValue: "yellow"},
				{expectedKey: "lime", expectedValue: "green"},
			},
		},
		{
			desc: "#4 - single entry",
			inputUserKeys: []string{
				"strawberry",
			},
			inputValues: []string{
				"red",
			},
			restartInterval: 5,
			expectedSequence: []expectedEntry{
				{expectedKey: "strawberry", expectedValue: "red"},
			},
		},
		{
			desc: "#5 - large restart interval",
			inputUserKeys: []string{
				"orange", "papaya", "peach", "pear",
			},
			inputValues: []string{
				"orange", "orange", "pink", "green",
			},
			restartInterval: 4,
			expectedSequence: []expectedEntry{
				{expectedKey: "orange", expectedValue: "orange"},
				{expectedKey: "papaya", expectedValue: "orange"},
				{expectedKey: "peach", expectedValue: "pink"},
				{expectedKey: "pear", expectedValue: "green"},
			},
		},
		{
			desc: "#6 - empty values",
			inputUserKeys: []string{
				"key1", "key2", "key3",
			},
			inputValues: []string{
				"", "", "",
			},
			restartInterval: 2,
			expectedSequence: []expectedEntry{
				{expectedKey: "key1", expectedValue: ""},
				{expectedKey: "key2", expectedValue: ""},
				{expectedKey: "key3", expectedValue: ""},
			},
		},
		{
			desc: "#7 - keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval: 3,
			expectedSequence: []expectedEntry{
				{expectedKey: "prefix_key_001", expectedValue: "value1"},
				{expectedKey: "prefix_key_002", expectedValue: "value2"},
				{expectedKey: "prefix_key_003", expectedValue: "value3"},
				{expectedKey: "prefix_key_004", expectedValue: "value4"},
			},
		},
		{
			desc: "#8 - two entries only",
			inputUserKeys: []string{
				"first", "second",
			},
			inputValues: []string{
				"value1", "value2",
			},
			restartInterval: 3,
			expectedSequence: []expectedEntry{
				{expectedKey: "first", expectedValue: "value1"},
				{expectedKey: "second", expectedValue: "value2"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data using the existing writer
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test sequential Next() calls
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			firstKV := iter.First()
			assert.NotNil(t, firstKV, "First() should return non-nil InternalKV")
			assert.Equal(t, tc.expectedSequence[0].expectedKey, string(firstKV.K.UserKey), "First() should return the first key")
			assert.Equal(t, tc.expectedSequence[0].expectedValue, string(firstKV.V.Value()), "First() should return the first value")

			// Test Next() for remaining entries
			for i := 1; i < len(tc.expectedSequence); i++ {
				expected := tc.expectedSequence[i]

				assert.Less(t, iter.nextOffset, iter.trailerOffset, "nextOffset should be less than trailer before Next() call %d", i)

				nextKV := iter.Next()

				assert.NotNil(t, nextKV, "Next() should return non-nil InternalKV for entry %d", i)
				assert.Equal(t, expected.expectedKey, string(nextKV.K.UserKey), "Next() should return correct key for entry %d", i)
				assert.Equal(t, expected.expectedValue, string(nextKV.V.Value()), "Next() should return correct value for entry %d", i)

				// Verify iterator internal state
				actualIterKey := nogodb_common.DeserializeKey(iter.key)
				assert.Equal(t, expected.expectedKey, string(actualIterKey.UserKey), "iterator's internal key should match for entry %d", i)
				assert.Equal(t, expected.expectedValue, string(iter.value), "iterator's internal value should match for entry %d", i)

				// Verify offset progression
				if i < len(tc.expectedSequence)-1 {
					assert.Greater(t, iter.nextOffset, iter.offset, "nextOffset should be greater than current offset for entry %d", i)
					assert.LessOrEqual(t, iter.nextOffset, iter.trailerOffset, "nextOffset should not exceed trailer offset for entry %d", i)
				}
			}

			// Verify we've read all entries and are at or near the trailer
			assert.LessOrEqual(t, iter.nextOffset, iter.trailerOffset, "final nextOffset should not exceed trailer offset")

			// If we trigger the next() at the last position, the returned internalKV is nil
			nextKV := iter.Next()
			assert.Nil(t, nextKV)
		})
	}
}

func TestDataBlockIterator_Last(t *testing.T) {
	type testCase struct {
		desc              string
		inputUserKeys     []string
		inputValues       []string
		restartInterval   int
		expectedLastKey   string
		expectedLastValue string
	}

	tests := []testCase{
		{
			desc: "#1 - restart interval 2",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "avowed", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "purple", "yellow", "red", "blue",
			},
			restartInterval:   2,
			expectedLastKey:   "mango",
			expectedLastValue: "blue",
		},
		{
			desc: "#2 - restart interval 1",
			inputUserKeys: []string{
				"banana", "berry", "blueberry",
			},
			inputValues: []string{
				"yellow", "red", "blue",
			},
			restartInterval:   1,
			expectedLastKey:   "blueberry",
			expectedLastValue: "blue",
		},
		{
			desc: "#3 - restart interval 3",
			inputUserKeys: []string{
				"grape", "grapefruit", "guava", "kiwi", "lemon", "lime",
			},
			inputValues: []string{
				"purple", "pink", "green", "brown", "yellow", "green",
			},
			restartInterval:   3,
			expectedLastKey:   "lime",
			expectedLastValue: "green",
		},
		{
			desc: "#4 - single entry",
			inputUserKeys: []string{
				"strawberry",
			},
			inputValues: []string{
				"red",
			},
			restartInterval:   5,
			expectedLastKey:   "strawberry",
			expectedLastValue: "red",
		},
		{
			desc: "#5 - large restart interval",
			inputUserKeys: []string{
				"orange", "papaya", "peach", "pear",
			},
			inputValues: []string{
				"orange", "orange", "pink", "green",
			},
			restartInterval:   4,
			expectedLastKey:   "pear",
			expectedLastValue: "green",
		},
		{
			desc: "#6 - empty last value",
			inputUserKeys: []string{
				"key1", "key2", "key3",
			},
			inputValues: []string{
				"value1", "value2", "",
			},
			restartInterval:   2,
			expectedLastKey:   "key3",
			expectedLastValue: "",
		},
		{
			desc: "#7 - keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval:   3,
			expectedLastKey:   "prefix_key_004",
			expectedLastValue: "value4",
		},
		{
			desc: "#8 - very long last key and value",
			inputUserKeys: []string{
				"short",
				"medium_key",
				"verylongkeyname1234567890abcdefghijklmnopqrstuvwxyz",
			},
			inputValues: []string{
				"small",
				"medium_value",
				"verylongvaluecontent1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			},
			restartInterval:   2,
			expectedLastKey:   "verylongkeyname1234567890abcdefghijklmnopqrstuvwxyz",
			expectedLastValue: "verylongvaluecontent1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
		{
			desc: "#9 - two entries only",
			inputUserKeys: []string{
				"first", "second",
			},
			inputValues: []string{
				"value1", "value2",
			},
			restartInterval:   3,
			expectedLastKey:   "second",
			expectedLastValue: "value2",
		},
		{
			desc: "#10 - restart interval equals number of entries",
			inputUserKeys: []string{
				"alpha", "beta", "gamma", "delta",
			},
			inputValues: []string{
				"first", "second", "third", "fourth",
			},
			restartInterval:   4,
			expectedLastKey:   "delta",
			expectedLastValue: "fourth",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data using the existing writer
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test Last()
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			lastKV := iter.Last()

			assert.NotNil(t, lastKV, "Last() should return non-nil InternalKV")
			assert.Equal(t, tc.expectedLastKey, string(lastKV.K.UserKey), "Last() should return the last key")
			assert.Equal(t, tc.expectedLastValue, string(lastKV.V.Value()), "Last() should return the last value")
			assert.Equal(t, iter.trailerOffset, iter.nextOffset, "nextOffset should equal trailerOffset after Last()")

			// Verify iterator's internal key and value are set to last entry
			actualIterKey := nogodb_common.DeserializeKey(iter.key)
			assert.Equal(t, tc.expectedLastKey, string(actualIterKey.UserKey), "iterator's internal key should match last key")
			assert.Equal(t, tc.expectedLastValue, string(iter.value), "iterator's internal value should match last value")

			// Verify that Last() positions iterator at the correct restart point or later
			expectedLastRestartPoint := iter.restartPoints[len(iter.restartPoints)-1]
			assert.GreaterOrEqual(t, iter.offset, uint64(expectedLastRestartPoint), "iterator offset should be at or after the last restart point")
		})
	}
}

func TestDataBlockIterator_Prev(t *testing.T) {
	type testCase struct {
		desc              string
		inputUserKeys     []string
		inputValues       []string
		restartInterval   int
		initialPosition   int
		expectedPrevKey   string
		expectedPrevValue string
	}

	tests := []testCase{
		{
			desc: "#1 - Prev() from first entry (should return same)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval: 2,
			initialPosition: 0,
		},
		{
			desc: "#2 - Prev() from second entry",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:   2,
			initialPosition:   1,
			expectedPrevKey:   "apple",
			expectedPrevValue: "red",
		},
		{
			desc: "#3 - Prev() from middle entry",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:   2,
			initialPosition:   2,
			expectedPrevKey:   "apricot",
			expectedPrevValue: "orange",
		},
		{
			desc: "#4 - Prev() from restart point",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango", "orange",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow", "orange",
			},
			restartInterval:   3,
			initialPosition:   3,
			expectedPrevKey:   "avocado",
			expectedPrevValue: "green",
		},
		{
			desc: "#5 - Prev() from last entry",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:   2,
			initialPosition:   4,
			expectedPrevKey:   "cherry",
			expectedPrevValue: "red",
		},
		{
			desc: "#6 - Single entry block",
			inputUserKeys: []string{
				"only",
			},
			inputValues: []string{
				"value",
			},
			restartInterval: 1,
			initialPosition: 0,
		},
		{
			desc: "#7 - Prev() with restart interval 1 (all entries are restart points)",
			inputUserKeys: []string{
				"alpha", "beta", "gamma", "delta",
			},
			inputValues: []string{
				"first", "second", "third", "fourth",
			},
			restartInterval:   1,
			initialPosition:   2,
			expectedPrevKey:   "beta",
			expectedPrevValue: "second",
		},
		{
			desc: "#8 - Prev() with large restart interval",
			inputUserKeys: []string{
				"a", "b", "c", "d", "e", "f",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5", "6",
			},
			restartInterval:   6, // only one restart point at beginning
			initialPosition:   5,
			expectedPrevKey:   "e",
			expectedPrevValue: "5",
		},
		{
			desc: "#9 - Prev() from second restart point",
			inputUserKeys: []string{
				"apple", "banana", "cherry", "date", "elderberry", "fig",
			},
			inputValues: []string{
				"red", "yellow", "red", "brown", "purple", "green",
			},
			restartInterval:   2, // restart points at 0, 2, 4
			initialPosition:   4,
			expectedPrevKey:   "date",
			expectedPrevValue: "brown",
		},
		{
			desc: "#10 - Two entries, Prev() from second",
			inputUserKeys: []string{
				"first", "second",
			},
			inputValues: []string{
				"value1", "value2",
			},
			restartInterval:   1,
			initialPosition:   1,
			expectedPrevKey:   "first",
			expectedPrevValue: "value1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and position it correctly
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			// Position iterator based on test case
			iter.First()
			for range tc.initialPosition {
				iter.Next()
			}

			// 3. Call Prev() and verify results
			prevKV := iter.Prev()

			if len(tc.expectedPrevKey) == 0 {
				assert.Nil(t, prevKV, "Prev() should return nil InternalKV")
			} else {
				assert.NotNil(t, prevKV, "Prev() should return non-nil InternalKV")
				assert.Equal(t, tc.expectedPrevKey, string(prevKV.K.UserKey), "Prev() should return correct key")
				assert.Equal(t, tc.expectedPrevValue, string(prevKV.V.Value()), "Prev() should return correct value")

				// Verify iterator's internal state
				actualIterKey := nogodb_common.DeserializeKey(iter.key)
				assert.Equal(t, tc.expectedPrevKey, string(actualIterKey.UserKey), "iterator's internal key should match")
				assert.Equal(t, tc.expectedPrevValue, string(iter.value), "iterator's internal value should match")
			}
		})
	}
}

func TestDataBlockIterator_SeekGTE(t *testing.T) {
	type testCase struct {
		desc               string
		inputUserKeys      []string
		inputValues        []string
		restartInterval    int
		seekKey            string
		expectedFoundKey   string
		expectedFoundValue string
		isNotFound         bool
	}

	tests := []testCase{
		{
			desc: "#1 - Seek exact match at first key",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "apple",
			expectedFoundKey:   "apple",
			expectedFoundValue: "red",
		},
		{
			desc: "#2 - Seek exact match at middle key",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "avocado",
			expectedFoundKey:   "avocado",
			expectedFoundValue: "green",
		},
		{
			desc: "#3 - Seek exact match at last key",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "mango",
			expectedFoundKey:   "mango",
			expectedFoundValue: "yellow",
		},
		{
			desc: "#4 - Seek between keys (should find next greater key)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "car",    // "car" > "avocado" but < "cherry"
			expectedFoundKey:   "cherry", // first key >= "car"
			expectedFoundValue: "red",
		},
		{
			desc: "#5 - Seek key smaller than first (should return first)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "aaa", // before "apple"
			expectedFoundKey:   "apple",
			expectedFoundValue: "red",
		},
		{
			desc: "#6 - Seek key larger than last",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval: 2,
			seekKey:         "zebra", // after "mango"
			isNotFound:      true,
		},
		{
			desc: "#7 - Seek at restart point",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango", "orange",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow", "orange",
			},
			restartInterval:    3, // restart points at 0, 3 (cherry)
			seekKey:            "cherry",
			expectedFoundKey:   "cherry",
			expectedFoundValue: "red",
		},
		{
			desc: "#8 - Single entry block",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval:    1,
			seekKey:            "single",
			expectedFoundKey:   "single",
			expectedFoundValue: "value",
		},
		{
			desc: "#9 - Single entry block, seek smaller",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval:    1,
			seekKey:            "aaa",
			expectedFoundKey:   "single",
			expectedFoundValue: "value",
		},
		{
			desc: "#10 - Single entry block, seek larger",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval: 1,
			seekKey:         "zzz",
			isNotFound:      true,
		},
		{
			desc: "#11 - Keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval:    2,
			seekKey:            "prefix_key_002",
			expectedFoundKey:   "prefix_key_002",
			expectedFoundValue: "value2",
		},
		{
			desc: "#12 - Seek between prefixed keys",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_003", "prefix_key_005", "prefix_key_007",
			},
			inputValues: []string{
				"value1", "value3", "value5", "value7",
			},
			restartInterval:    2,
			seekKey:            "prefix_key_004", // between 003 and 005
			expectedFoundKey:   "prefix_key_005",
			expectedFoundValue: "value5",
		},
		{
			desc: "#13 - Restart interval 1 (all entries are restart points)",
			inputUserKeys: []string{
				"alpha", "beta", "delta", "epsilon", "gamma",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5",
			},
			restartInterval:    1,
			seekKey:            "delta",
			expectedFoundKey:   "delta",
			expectedFoundValue: "3",
		},
		{
			desc: "#14 - Large restart interval",
			inputUserKeys: []string{
				"a", "b", "c", "d", "e", "f", "g", "h",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5", "6", "7", "8",
			},
			restartInterval:    8, // single restart point
			seekKey:            "e",
			expectedFoundKey:   "e",
			expectedFoundValue: "5",
		},
		{
			desc: "#15 - Seek between restart points",
			inputUserKeys: []string{
				"apple", "banana", "cherry", "date", "elderberry", "fig",
			},
			inputValues: []string{
				"red", "yellow", "red", "brown", "purple", "green",
			},
			restartInterval:    2,         // restart points at 0, 2, 4
			seekKey:            "coconut", // between cherry and date
			expectedFoundKey:   "date",
			expectedFoundValue: "brown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test SeekGTE
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			seekKey := makeDummyKey(tc.seekKey)
			buf := make([]byte, seekKey.Size())
			seekKey.SerializeTo(buf)
			seekKV := iter.SeekGTE(buf)

			// 3. Verify results
			if tc.isNotFound {
				assert.Nil(t, seekKV)
			} else {
				assert.Equal(t, tc.expectedFoundKey, string(seekKV.K.UserKey), "SeekGTE() should return correct key")
				assert.Equal(t, tc.expectedFoundValue, string(seekKV.V.Value()), "SeekGTE() should return correct value")

				// Verify iterator's internal state matches
				actualIterKey := nogodb_common.DeserializeKey(iter.key)
				assert.Equal(t, tc.expectedFoundKey, string(actualIterKey.UserKey), "iterator's internal key should match found key")
				assert.Equal(t, tc.expectedFoundValue, string(iter.value), "iterator's internal value should match found value")

				// Verify that the found key is indeed >= seek key
				cmpResult := cmp.Compare([]byte(tc.expectedFoundKey), []byte(tc.seekKey))
				assert.GreaterOrEqual(t, cmpResult, 0, "found key should be >= seek key")

				// Additional verification: ensure this is the first key >= seek key
				// by checking that there's no smaller key >= seek key in our input
				foundIndex := -1
				for i, inputKey := range tc.inputUserKeys {
					if inputKey == tc.expectedFoundKey {
						foundIndex = i
						break
					}
				}
				assert.NotEqual(t, -1, foundIndex, "found key should exist in input keys")

				// Verify this is truly the first key >= seek key by checking no earlier key qualifies
				for i := range foundIndex {
					prevKeyCmp := cmp.Compare([]byte(tc.inputUserKeys[i]), []byte(tc.seekKey))
					assert.Less(t, prevKeyCmp, 0, "all keys before found key should be < seek key, but key %s at index %d is not < %s", tc.inputUserKeys[i], i, tc.seekKey)
				}
			}
		})
	}
}

func TestBlockIterator_Close(t *testing.T) {
	type testCase struct {
		desc            string
		inputUserKeys   []string
		inputValues     []string
		restartInterval int
		setupOperation  string // "none", "first", "last", "next", "seekgte", "multiple"
	}

	tests := []testCase{
		{
			desc: "#1 - Close after initialization only",
			inputUserKeys: []string{
				"apple", "banana", "cherry",
			},
			inputValues: []string{
				"red", "yellow", "red",
			},
			restartInterval: 2,
			setupOperation:  "none",
		},
		{
			desc: "#2 - Close after First()",
			inputUserKeys: []string{
				"apple", "banana", "cherry", "date",
			},
			inputValues: []string{
				"red", "yellow", "red", "brown",
			},
			restartInterval: 2,
			setupOperation:  "first",
		},
		{
			desc: "#3 - Close after Last()",
			inputUserKeys: []string{
				"alpha", "beta", "gamma", "delta", "epsilon",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5",
			},
			restartInterval: 3,
			setupOperation:  "last",
		},
		{
			desc: "#4 - Close after Next() operations",
			inputUserKeys: []string{
				"key1", "key2", "key3", "key4", "key5", "key6",
			},
			inputValues: []string{
				"val1", "val2", "val3", "val4", "val5", "val6",
			},
			restartInterval: 2,
			setupOperation:  "next",
		},
		{
			desc: "#5 - Close after SeekGTE()",
			inputUserKeys: []string{
				"apple", "apricot", "banana", "cherry", "grape", "mango",
			},
			inputValues: []string{
				"red", "orange", "yellow", "red", "purple", "yellow",
			},
			restartInterval: 2,
			setupOperation:  "seekgte",
		},
		{
			desc: "#6 - Close after multiple operations",
			inputUserKeys: []string{
				"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
			},
			restartInterval: 3,
			setupOperation:  "multiple",
		},
		{
			desc: "#7 - Close with single entry block",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval: 1,
			setupOperation:  "first",
		},
		{
			desc: "#8 - Close with large restart interval",
			inputUserKeys: []string{
				"key1", "key2", "key3", "key4", "key5",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4", "value5",
			},
			restartInterval: 5, // All entries in one restart segment
			setupOperation:  "multiple",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			// Capture initial state to verify it gets reset
			assert.NotNil(t, iter.data, "data should not be nil before Close()")
			assert.False(t, iter.IsClosed(), "iterator should not be closed initially")

			// 3. Perform setup operations based on test case
			switch tc.setupOperation {
			case "first":
				kv := iter.First()
				assert.NotNil(t, kv, "First() should return non-nil")
				assert.NotNil(t, iter.key, "key should be set after First()")
				assert.NotNil(t, iter.value, "value should be set after First()")
				assert.Greater(t, iter.nextOffset, uint64(0), "nextOffset should be > 0 after First()")

			case "last":
				kv := iter.Last()
				assert.NotNil(t, kv, "Last() should return non-nil")
				assert.NotNil(t, iter.key, "key should be set after Last()")
				assert.NotNil(t, iter.value, "value should be set after Last()")
				assert.Equal(t, iter.trailerOffset, iter.nextOffset, "nextOffset should equal trailerOffset after Last()")

			case "next":
				iter.First()
				for i := 0; i < 2 && iter.nextOffset < iter.trailerOffset; i++ {
					kv := iter.Next()
					assert.NotNil(t, kv, "Next() should return non-nil")
				}
				assert.NotNil(t, iter.key, "key should be set after Next() operations")
				assert.NotNil(t, iter.value, "value should be set after Next() operations")

			case "seekgte":
				seekKey := tc.inputUserKeys[len(tc.inputUserKeys)/2] // Seek to middle key
				kv := iter.SeekGTE([]byte(seekKey))
				assert.NotNil(t, kv, "SeekGTE() should return non-nil")
				assert.NotNil(t, iter.key, "key should be set after SeekGTE()")
				assert.NotNil(t, iter.value, "value should be set after SeekGTE()")

			case "multiple":
				// Perform a series of operations
				iter.First()
				iter.Next()
				iter.Last()
				seekKey := tc.inputUserKeys[0]
				iter.SeekGTE([]byte(seekKey))
				assert.NotNil(t, iter.key, "key should be set after multiple operations")
				assert.NotNil(t, iter.value, "value should be set after multiple operations")

			case "none":
				// No operations, just verify initial state
				// Iterator should still be in initial state
			}

			// Store references to verify cleanup (before they get set to nil)
			originalData := iter.data
			hadKey := len(iter.key) > 0
			hadValue := len(iter.value) > 0
			originalOffset := iter.offset
			originalNextOffset := iter.nextOffset
			originalNumRestarts := iter.numRestarts
			hadRestartPoints := len(iter.restartPoints) > 0

			// 4. Call Close() and verify return value
			err := iter.Close()
			assert.NoError(t, err, "Close() should not return an error")

			// 5. Verify all fields are properly reset
			assert.Nil(t, iter.data, "data should be nil after Close()")
			assert.Nil(t, iter.key, "key should be nil after Close()")
			assert.Nil(t, iter.value, "value should be nil after Close()")
			assert.Equal(t, uint64(0), iter.offset, "offset should be 0 after Close()")
			assert.Equal(t, uint64(0), iter.nextOffset, "nextOffset should be 0 after Close()")
			assert.Equal(t, int32(0), iter.numRestarts, "numRestarts should be 0 after Close()")
			assert.Nil(t, iter.restartPoints, "restartPoints should be nil after Close()")

			// 6. Verify IsClosed() returns true
			assert.True(t, iter.IsClosed(), "IsClosed() should return true after Close()")

			// 7. Verify that the fields were actually set before Close() (for non-"none" cases)
			if tc.setupOperation != "none" {
				assert.NotNil(t, originalData, "original data should not have been nil")
				if tc.setupOperation != "none" {
					// For operations that set key/value, verify they were set
					switch tc.setupOperation {
					case "first", "last", "next", "seekgte", "multiple":
						assert.True(t, hadKey, "key should have been set before Close() for operation: %s", tc.setupOperation)
						assert.True(t, hadValue, "value should have been set before Close() for operation: %s", tc.setupOperation)
					}
				}
			}

			// Store original values for verification
			_ = originalOffset
			_ = originalNextOffset
			_ = originalNumRestarts
			_ = hadRestartPoints

			// 8. Verify all fields remain nil/zero after second Close()
			assert.Nil(t, iter.data, "data should remain nil after second Close()")
			assert.Nil(t, iter.key, "key should remain nil after second Close()")
			assert.Nil(t, iter.value, "value should remain nil after second Close()")
			assert.Equal(t, uint64(0), iter.offset, "offset should remain 0 after second Close()")
			assert.Equal(t, uint64(0), iter.nextOffset, "nextOffset should remain 0 after second Close()")
			assert.Equal(t, int32(0), iter.numRestarts, "numRestarts should remain 0 after second Close()")
			assert.Nil(t, iter.restartPoints, "restartPoints should remain nil after second Close()")
		})
	}
}

func TestDataBlockIterator_SeekLTE(t *testing.T) {
	type testCase struct {
		desc               string
		inputUserKeys      []string
		inputValues        []string
		restartInterval    int
		seekKey            string
		expectedFoundKey   string
		expectedFoundValue string
		isNotFound         bool
	}

	tests := []testCase{
		{
			desc: "#1 - Seek key smaller than first (should return nil)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval: 2,
			seekKey:         "aaa", // before "apple"
			isNotFound:      true,
		},
		{
			desc: "#2 - Seek key larger than last (should return last)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "zebra", // after "mango"
			expectedFoundKey:   "mango",
			expectedFoundValue: "yellow",
		},
		{
			desc: "#3 - Seek exact match",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "cherry",
			expectedFoundKey:   "cherry",
			expectedFoundValue: "red",
		},
		{
			desc: "#4 - Seek between keys (should find largest key < target)",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "car", // between "avocado" and "cherry"
			expectedFoundKey:   "avocado",
			expectedFoundValue: "green",
		},
		{
			desc: "#5 - Seek exact match at first key",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "apple",
			expectedFoundKey:   "apple",
			expectedFoundValue: "red",
		},
		{
			desc: "#6 - Seek exact match at last key",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow",
			},
			restartInterval:    2,
			seekKey:            "mango",
			expectedFoundKey:   "mango",
			expectedFoundValue: "yellow",
		},
		{
			desc: "#7 - Seek at restart point",
			inputUserKeys: []string{
				"apple", "apricot", "avocado", "cherry", "mango", "orange",
			},
			inputValues: []string{
				"red", "orange", "green", "red", "yellow", "orange",
			},
			restartInterval:    3, // restart points at 0, 3 (cherry)
			seekKey:            "cherry",
			expectedFoundKey:   "cherry",
			expectedFoundValue: "red",
		},
		{
			desc: "#8 - Single entry block, seek same key",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval:    1,
			seekKey:            "single",
			expectedFoundKey:   "single",
			expectedFoundValue: "value",
		},
		{
			desc: "#9 - Single entry block, seek smaller",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval: 1,
			seekKey:         "aaa",
			isNotFound:      true,
		},
		{
			desc: "#10 - Single entry block, seek larger",
			inputUserKeys: []string{
				"single",
			},
			inputValues: []string{
				"value",
			},
			restartInterval:    1,
			seekKey:            "zzz",
			expectedFoundKey:   "single",
			expectedFoundValue: "value",
		},
		{
			desc: "#11 - Keys with common prefixes",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_002", "prefix_key_003", "prefix_key_004",
			},
			inputValues: []string{
				"value1", "value2", "value3", "value4",
			},
			restartInterval:    2,
			seekKey:            "prefix_key_003",
			expectedFoundKey:   "prefix_key_003",
			expectedFoundValue: "value3",
		},
		{
			desc: "#12 - Seek between prefixed keys",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_003", "prefix_key_005", "prefix_key_007",
			},
			inputValues: []string{
				"value1", "value3", "value5", "value7",
			},
			restartInterval:    2,
			seekKey:            "prefix_key_004", // between 003 and 005
			expectedFoundKey:   "prefix_key_003",
			expectedFoundValue: "value3",
		},
		{
			desc: "#13 - Restart interval 1 (all entries are restart points)",
			inputUserKeys: []string{
				"alpha", "beta", "delta", "epsilon", "gamma",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5",
			},
			restartInterval:    1,
			seekKey:            "delta",
			expectedFoundKey:   "delta",
			expectedFoundValue: "3",
		},
		{
			desc: "#14 - Large restart interval",
			inputUserKeys: []string{
				"a", "b", "c", "d", "e", "f", "g", "h",
			},
			inputValues: []string{
				"1", "2", "3", "4", "5", "6", "7", "8",
			},
			restartInterval:    8, // single restart point
			seekKey:            "e",
			expectedFoundKey:   "e",
			expectedFoundValue: "5",
		},
		{
			desc: "#15 - Seek between restart points",
			inputUserKeys: []string{
				"apple", "banana", "cherry", "date", "elderberry", "fig",
			},
			inputValues: []string{
				"red", "yellow", "red", "brown", "purple", "green",
			},
			restartInterval:    2,         // restart points at 0, 2, 4
			seekKey:            "coconut", // between cherry and date
			expectedFoundKey:   "cherry",
			expectedFoundValue: "red",
		},
		{
			desc: "#16 - Two entries, seek second",
			inputUserKeys: []string{
				"first", "second",
			},
			inputValues: []string{
				"value1", "value2",
			},
			restartInterval:    1,
			seekKey:            "second",
			expectedFoundKey:   "second",
			expectedFoundValue: "value2",
		},
		{
			desc: "#17 - Two entries, seek between",
			inputUserKeys: []string{
				"first", "third",
			},
			inputValues: []string{
				"value1", "value3",
			},
			restartInterval:    1,
			seekKey:            "second", // between first and third
			expectedFoundKey:   "first",
			expectedFoundValue: "value1",
		},
		{
			desc: "#18 - Seek key that is out of range",
			inputUserKeys: []string{
				"prefix_key_001", "prefix_key_003", "prefix_key_005", "prefix_key_007",
			},
			inputValues: []string{
				"value1", "value3", "value5", "value7",
			},
			restartInterval:    2,
			seekKey:            "prefix_key_008",
			expectedFoundKey:   "prefix_key_007",
			expectedFoundValue: "value7",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// 1. Create block data
			bp := predictable_size.NewPredictablePool()
			blk := newBlock(tc.restartInterval, bp, 1024*1024)

			for i, userKey := range tc.inputUserKeys {
				key := makeDummyKey(userKey)
				value := []byte(tc.inputValues[i])
				err := blk.WriteEntry(key, value)
				assert.NoError(t, err, "should not have error writing entry")
			}

			blockData := make([]byte, blk.EstimateSize())
			blk.Finish(blockData)

			// 2. Create iterator and test SeekLT
			cmp := nogodb_common.NewComparer()
			lz := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
			lz.ReserveBuffer(bp, len(blockData))
			_ = lz.SetBufferValue(blockData)
			iter := NewBlockIterator(predictable_size.NewPredictablePool(), cmp, &lz)

			seekKey := makeDummyKey(tc.seekKey)
			buf := make([]byte, seekKey.Size())
			seekKey.SerializeTo(buf)
			seekKV := iter.SeekLTE(buf)

			// 3. Verify results
			if tc.isNotFound {
				assert.Nil(t, seekKV, "SeekLT() should return nil when no key < seek key")
			} else {
				assert.NotNil(t, seekKV, "SeekLT() should return non-nil InternalKV")
				assert.Equal(t, tc.expectedFoundKey, string(seekKV.K.UserKey), "SeekLT() should return correct key")
				assert.Equal(t, tc.expectedFoundValue, string(seekKV.V.Value()), "SeekLT() should return correct value")

				// Verify iterator's internal state matches
				actualIterKey := nogodb_common.DeserializeKey(iter.key)
				assert.Equal(t, tc.expectedFoundKey, string(actualIterKey.UserKey), "iterator's internal key should match found key")
				assert.Equal(t, tc.expectedFoundValue, string(iter.value), "iterator's internal value should match found value")

				// Verify that the found key is indeed <= seek key
				cmpResult := cmp.Compare([]byte(tc.expectedFoundKey), []byte(tc.seekKey))
				assert.LessOrEqual(t, cmpResult, 0, "found key should be <= seek key")

				// Additional verification: ensure this is the largest key < seek key
				// by checking that there's no larger key < seek key in our input
				foundIndex := -1
				for i, inputKey := range tc.inputUserKeys {
					if inputKey == tc.expectedFoundKey {
						foundIndex = i
						break
					}
				}
				assert.NotEqual(t, -1, foundIndex, "found key should exist in input keys")

				// Check that all subsequent keys are >= seek key (if any)
				for i := foundIndex + 1; i < len(tc.inputUserKeys); i++ {
					nextKeyCmp := cmp.Compare([]byte(tc.inputUserKeys[i]), []byte(tc.seekKey))
					assert.GreaterOrEqual(t, nextKeyCmp, 0, "all keys after found key should be >= seek key, but key %s at index %d is not", tc.inputUserKeys[i], i)
				}
			}
		})
	}
}
