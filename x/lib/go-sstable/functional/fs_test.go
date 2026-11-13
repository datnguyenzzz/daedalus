//go:build functional_tests

package functional

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	go_block_cache "github.com/datnguyenzzz/nogodb/lib/go-block-cache"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	go_sstable "github.com/datnguyenzzz/nogodb/lib/go-sstable"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

const (
	dirname     = ".nogodb"
	bytePerSync = 2 * 1024 * 1024
)

type SSTSuiteFS struct {
	suite.Suite
	storage go_fs.Storage
}

func (w *SSTSuiteFS) SetupSubTest() {
	var err error
	w.storage, err = go_fs.OpenVfsProvider(
		go_fs.WithDirName(dirname),
		go_fs.WithBytesPerSync(bytePerSync),
		go_fs.WithFS(go_fs.NewDefaultUnix()),
	)
	require.Nil(w.T(), err)
}

func (w *SSTSuiteFS) TearDownSubTest() {
	err := w.storage.Close()
	require.Nil(w.T(), err)

	// Remove all page data files
	files, _ := os.ReadDir(dirname)
	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(dirname, file.Name())
			_ = os.Remove(filePath)
		}
	}

	_ = os.Remove(dirname)
}

func (w *SSTSuiteFS) Test_Integration_Writer_No_Errors() {
	type param struct {
		name      string
		restart   int
		blockSize int
		isUnique  bool
		operation string
	}

	tests := []param{
		{
			name:      "small block, all keys are unique",
			isUnique:  true,
			restart:   5,
			blockSize: 2 * kB,
			operation: "SET",
		},
		{
			name:      "small block, no unique are unique",
			restart:   5,
			blockSize: 2 * kB,
			operation: "SET",
		},
		{
			name:      "big block, all keys are unique",
			isUnique:  true,
			restart:   10,
			blockSize: 4 * mB,
			operation: "SET",
		},
		{
			name:      "big block, no unique are unique",
			restart:   10,
			blockSize: 4 * mB,
			operation: "SET",
		},
		{
			name:      "small block, all keys are unique",
			isUnique:  true,
			restart:   5,
			blockSize: 2 * kB,
			operation: "DEL",
		},
		{
			name:      "small block, no unique are unique",
			restart:   5,
			blockSize: 2 * kB,
			operation: "DEL",
		},
		{
			name:      "big block, all keys are unique",
			isUnique:  true,
			restart:   10,
			blockSize: 4 * mB,
			operation: "DEL",
		},
		{
			name:      "big block, no unique are unique",
			restart:   10,
			blockSize: 4 * mB,
			operation: "DEL",
		},
	}

	for i, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			assert.NoError(t, err)
			writer := go_sstable.NewWriter(
				fileWritable,
				common.TableV1,
				go_sstable.WithBlockRestartInterval(tc.restart),
				go_sstable.WithBlockSize(tc.blockSize),
			)

			sample := generateKV(testSize, tc.isUnique)
			for _, kv := range sample {
				switch tc.operation {
				case "SET":
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				case "DEL":
					err := writer.Delete(kv.key)
					assert.NoError(t, err, "failed to delete")
				}
			}

			err = writer.Close()
			assert.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Integration_Writer_No_Errors_MVCC_col_block() {
	type param struct {
		name      string
		restart   int
		blockSize int
		isUnique  bool
		operation string
	}

	tests := []param{
		{
			name:      "small block, all keys are unique",
			isUnique:  true,
			restart:   5,
			blockSize: 2 * kB,
			operation: "SET",
		},
		{
			name:      "small block, no unique are unique",
			restart:   5,
			blockSize: 2 * kB,
			operation: "SET",
		},
		{
			name:      "big block, all keys are unique",
			isUnique:  true,
			restart:   10,
			blockSize: 4 * mB,
			operation: "SET",
		},
		{
			name:      "big block, no unique are unique",
			restart:   10,
			blockSize: 4 * mB,
			operation: "SET",
		},
		{
			name:      "small block, all keys are unique",
			isUnique:  true,
			restart:   5,
			blockSize: 2 * kB,
			operation: "DEL",
		},
		{
			name:      "small block, no unique are unique",
			restart:   5,
			blockSize: 2 * kB,
			operation: "DEL",
		},
		{
			name:      "big block, all keys are unique",
			isUnique:  true,
			restart:   10,
			blockSize: 4 * mB,
			operation: "DEL",
		},
		{
			name:      "big block, no unique are unique",
			restart:   10,
			blockSize: 4 * mB,
			operation: "DEL",
		},
	}

	for i, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			assert.NoError(t, err)
			writer := go_sstable.NewWriter(
				fileWritable,
				common.TableV2,
				go_sstable.WithComparer(NewMvccComparer()),
				go_sstable.WithBlockRestartInterval(tc.restart),
				go_sstable.WithBlockSize(tc.blockSize),
			)

			sample := generateKVWithSuffix(testSize, tc.isUnique)
			for _, kv := range sample {
				switch tc.operation {
				case "SET":
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				case "DEL":
					err := writer.Delete(kv.key)
					assert.NoError(t, err, "failed to delete")
				}
			}

			err = writer.Close()
			assert.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_Seeking_Ops_single_table() {
	type param struct {
		name                string
		restart             int
		isUnique            bool
		cacheSize           int // 0 means no cache
		sampleSize          int
		isRandomisedOrdered bool
		cacheType           go_block_cache.CacheType
	}

	tests := []param{
		// Sequence read
		{
			name:       "volume = 1,  block cache disabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 1,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 1,  block cache enabled",
			isUnique:   true,
			restart:    5,
			cacheSize:  2 * kB,
			sampleSize: 1,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache disable",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache disable",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "ClockPro, volume = 1,  block cache enabled",
			isUnique:   true,
			restart:    5,
			cacheSize:  2 * kB,
			sampleSize: 1,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		// Randomised-order read
		{
			name:                "volume = 100_000, all keys are unique, block cache disable, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache disable, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "ClockPro, volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
		{
			name:                "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
	}

	for i, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			// Init a table
			fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			assert.NoError(t, err)
			writer := go_sstable.NewWriter(
				fileWritable,
				common.TableV1,
				go_sstable.WithBlockRestartInterval(tc.restart),
				go_sstable.WithBlockSize(2*kB),
			)

			sample := generateKV(tc.sampleSize, tc.isUnique)
			for _, kv := range sample {
				err := writer.Set(kv.key, kv.value)
				assert.NoError(t, err, "failed to set")
			}

			err = writer.Close()
			assert.NoError(t, err)

			// Evaluate the result of the seek operations
			fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			assert.NoError(t, err)
			var iterOpts []options.IteratorOptsFunc
			if tc.cacheSize > 0 {
				c := go_block_cache.NewMap(
					go_block_cache.WithMaxSize(int64(tc.cacheSize)),
					go_block_cache.WithCacheType(tc.cacheType),
					go_block_cache.WithShardNum(4),
				)
				iterOpts = []options.IteratorOptsFunc{
					options.WithBlockCache(c, fd),
				}
			}
			sharedBufferPool := predictable_size.NewPredictablePool()
			iter, err := go_sstable.NewSingularIterator(
				sharedBufferPool,
				fileReadable,
				iterOpts...,
			)
			require.NoError(t, err)

			defer func() {
				err := iter.Close()
				assert.NoError(t, err)
			}()

			for i := 0; i < len(sample); i++ {
				idx := i
				if tc.isRandomisedOrdered {
					r := rand.New(rand.NewSource(time.Now().UnixNano()))
					idx = int(r.Int31n(int32(len(sample))))
				}
				// SeekGTE with an exact key
				var prevKV kvType
				if idx > 0 {
					prevKV = sample[idx-1]
				}
				validateSeekGTE(t, iter, sample[idx], prevKV, idx)
			}

			// seek GTE with a key that is out of range of the SST
			k := make([]byte, len(sample[len(sample)-1].key))
			copy(k, sample[len(sample)-1].key)
			k[0] += 1
			kv := iter.SeekGTE(k)
			assert.Nil(t, kv)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_Seeking_Ops_single_table_MVCC_col_block() {
	type param struct {
		name                string
		isUnique            bool
		cacheSize           int // 0 means no cache
		sampleSize          int
		isRandomisedOrdered bool
		cacheType           go_block_cache.CacheType
	}

	tests := []param{
		// Sequence read
		{
			name:       "volume = 1,  block cache disabled",
			isUnique:   true,
			sampleSize: 1,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 1,  block cache enabled",
			isUnique:   true,
			cacheSize:  2 * kB,
			sampleSize: 1,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache disable",
			isUnique:   true,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache disable",
			isUnique:   false,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "ClockPro, volume = 1,  block cache enabled",
			isUnique:   true,
			cacheSize:  2 * kB,
			sampleSize: 1,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		// Randomised-order read
		{
			name:                "volume = 100_000, all keys are unique, block cache disable, randomised read order",
			isUnique:            true,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache disable, randomised read order",
			isUnique:            false,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "ClockPro, volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
		{
			name:                "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
	}

	for i, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()

			// Init a table
			fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			require.NoError(t, err)
			mvccComparer := NewMvccComparer()
			writer := go_sstable.NewWriter(
				fileWritable,
				common.TableV2,
				go_sstable.WithComparer(mvccComparer),
				go_sstable.WithBlockSize(2*kB),
			)

			sample := generateKVWithSuffix(tc.sampleSize, tc.isUnique)
			for _, kv := range sample {
				err := writer.Set(kv.key, kv.value)
				require.NoError(t, err, "failed to set")
			}

			err = writer.Close()
			require.NoError(t, err)

			// Evaluate the result of the seek operations
			fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(i))
			require.NoError(t, err)
			iterOpts := []options.IteratorOptsFunc{
				options.WithComparer(mvccComparer),
			}
			if tc.cacheSize > 0 {
				c := go_block_cache.NewMap(
					go_block_cache.WithMaxSize(int64(tc.cacheSize)),
					go_block_cache.WithCacheType(tc.cacheType),
					go_block_cache.WithShardNum(4),
				)
				iterOpts = append(iterOpts, []options.IteratorOptsFunc{
					options.WithBlockCache(c, fd),
				}...)
			}
			sharedBufferPool := predictable_size.NewPredictablePool()
			iter, err := go_sstable.NewSingularIterator(
				sharedBufferPool,
				fileReadable,
				iterOpts...,
			)
			require.NoError(t, err)

			for i := range sample {
				idx := i
				if tc.isRandomisedOrdered {
					r := rand.New(rand.NewSource(time.Now().UnixNano()))
					idx = int(r.Int31n(int32(len(sample))))
				}
				// SeekGTE with an exact key
				var prevKV kvType
				if idx > 0 {
					prevKV = sample[idx-1]
				}
				validateSeekGTE_MVCC_Key(t, iter, sample[idx], prevKV, idx)
			}

			// seek GTE with a key that is out of range of the SST
			k := make([]byte, len(sample[len(sample)-1].key))
			copy(k, sample[len(sample)-1].key)
			k[0] += 1
			kv := iter.SeekGTE(k)
			require.Nil(t, kv)

			err = iter.Close()
			require.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_Concurrently_Seeking_Ops_multiple_tables() {
	type param struct {
		name                string
		restart             int
		isUnique            bool
		cacheSize           int // 0 means no cache
		sampleSize          int
		isRandomisedOrdered bool
		cacheType           go_block_cache.CacheType
	}

	tests := []param{
		// Sequence read
		{
			name:       "volume = 100_000, all keys are unique, block cache disable",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache disable",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "ClockPro, volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		// Randomised-order read
		{
			name:                "volume = 100_000, all keys are unique, block cache disable, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache disable, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "ClockPro, volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
		{
			name:                "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
	}

	numberOfSSTs := 3

	for _, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			// Init a table
			sample := make([][]kvType, 0, numberOfSSTs)
			for sst := 1; sst <= numberOfSSTs; sst++ {
				fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
				assert.NoError(t, err)
				writer := go_sstable.NewWriter(
					fileWritable,
					common.TableV1,
					go_sstable.WithBlockRestartInterval(tc.restart),
					go_sstable.WithBlockSize(2*kB),
				)

				kvs := generateKV(tc.sampleSize, tc.isUnique)
				sample = append(sample, kvs)
				for _, kv := range kvs {
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				}

				err = writer.Close()
				assert.NoError(t, err)
			}

			// Evaluate the result of the seek operations
			eg := errgroup.Group{}
			for sst := 1; sst <= numberOfSSTs; sst++ {
				eg.Go(func() error {
					kvs := sample[sst-1]
					fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
					assert.NoError(t, err)
					var iterOpts []options.IteratorOptsFunc
					if tc.cacheSize > 0 {
						c := go_block_cache.NewMap(
							go_block_cache.WithMaxSize(int64(tc.cacheSize)),
							go_block_cache.WithCacheType(tc.cacheType),
							go_block_cache.WithShardNum(4),
						)
						iterOpts = []options.IteratorOptsFunc{
							options.WithBlockCache(c, fd),
						}
					}
					sharedBufferPool := predictable_size.NewPredictablePool()
					iter, err := go_sstable.NewSingularIterator(
						sharedBufferPool,
						fileReadable,
						iterOpts...,
					)
					require.NoError(t, err)

					defer func() {
						err := iter.Close()
						assert.NoError(t, err)
					}()

					for i := 0; i < len(kvs); i++ {
						idx := i
						if tc.isRandomisedOrdered {
							r := rand.New(rand.NewSource(time.Now().UnixNano()))
							idx = int(r.Int31n(int32(len(kvs))))
						}
						// SeekGTE with an exact key
						var prevKV kvType
						if idx > 0 {
							prevKV = kvs[idx-1]
						}
						validateSeekGTE(t, iter, kvs[idx], prevKV, idx)
					}

					// seek GTE with a key that is out of range of the SST
					k := make([]byte, len(kvs[len(kvs)-1].key))
					copy(k, kvs[len(kvs)-1].key)
					k[0] += 1
					kv := iter.SeekGTE(k)
					assert.Nil(t, kv)
					return nil
				})
			}

			err := eg.Wait()
			assert.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_Concurrently_Seeking_Ops_multiple_tables_MVCC_col_block() {
	type param struct {
		name                string
		restart             int
		isUnique            bool
		cacheSize           int // 0 means no cache
		sampleSize          int
		isRandomisedOrdered bool
		cacheType           go_block_cache.CacheType
	}

	tests := []param{
		// Sequence read
		{
			name:       "volume = 100_000, all keys are unique, block cache disable",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache disable",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.LRU,
		},
		{
			name:       "ClockPro, volume = 100_000, all keys are unique, block cache enabled",
			isUnique:   true,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		{
			name:       "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled",
			isUnique:   false,
			restart:    5,
			sampleSize: 100_000,
			cacheSize:  1 * mB,
			cacheType:  go_block_cache.ClockPro,
		},
		// Randomised-order read
		{
			name:                "volume = 100_000, all keys are unique, block cache disable, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache disable, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.LRU,
		},
		{
			name:                "ClockPro, volume = 100_000, all keys are unique, block cache enabled, randomised read order",
			isUnique:            true,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
		{
			name:                "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, randomised read order",
			isUnique:            false,
			restart:             5,
			sampleSize:          100_000,
			cacheSize:           1 * mB,
			isRandomisedOrdered: true,
			cacheType:           go_block_cache.ClockPro,
		},
	}

	numberOfSSTs := 3

	for _, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			// Init a table
			sample := make([][]kvType, 0, numberOfSSTs)
			mvccComparer := NewMvccComparer()

			for sst := 1; sst <= numberOfSSTs; sst++ {
				fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
				assert.NoError(t, err)
				writer := go_sstable.NewWriter(
					fileWritable,
					common.TableV2,
					go_sstable.WithBlockRestartInterval(tc.restart),
					go_sstable.WithBlockSize(2*kB),
					go_sstable.WithComparer(mvccComparer),
				)

				kvs := generateKVWithSuffix(tc.sampleSize, tc.isUnique)
				sample = append(sample, kvs)
				for _, kv := range kvs {
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				}

				err = writer.Close()
				assert.NoError(t, err)
			}
			// Evaluate the result of the seek operations
			eg := errgroup.Group{}
			for sst := 1; sst <= numberOfSSTs; sst++ {
				eg.Go(func() error {
					kvs := sample[sst-1]
					fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
					assert.NoError(t, err)
					iterOpts := []options.IteratorOptsFunc{
						options.WithComparer(mvccComparer),
					}
					if tc.cacheSize > 0 {
						c := go_block_cache.NewMap(
							go_block_cache.WithMaxSize(int64(tc.cacheSize)),
							go_block_cache.WithCacheType(tc.cacheType),
							go_block_cache.WithShardNum(4),
						)
						iterOpts = append(iterOpts, []options.IteratorOptsFunc{
							options.WithBlockCache(c, fd),
						}...)
					}
					sharedBufferPool := predictable_size.NewPredictablePool()
					iter, err := go_sstable.NewSingularIterator(
						sharedBufferPool,
						fileReadable,
						iterOpts...,
					)
					require.NoError(t, err)

					for i := range kvs {
						idx := i
						if tc.isRandomisedOrdered {
							r := rand.New(rand.NewSource(time.Now().UnixNano()))
							idx = int(r.Int31n(int32(len(kvs))))
						}
						// SeekGTE with an exact key
						var prevKV kvType
						if idx > 0 {
							prevKV = kvs[idx-1]
						}
						validateSeekGTE_MVCC_Key(t, iter, kvs[idx], prevKV, idx)
					}

					// seek GTE with a key that is out of range of the SST
					k := make([]byte, len(kvs[len(kvs)-1].key))
					copy(k, kvs[len(kvs)-1].key)
					k[0] += 1
					kv := iter.SeekGTE(k)
					require.Nil(t, kv)

					err = iter.Close()
					require.NoError(t, err)
					return nil
				})
			}

			err := eg.Wait()
			require.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_First_Then_Next_Ops() {
	type param struct {
		name      string
		restart   int
		isUnique  bool
		cacheSize int // 0 means no cache
		sstNum    int
		cacheType go_block_cache.CacheType
	}

	tests := []param{
		{
			name:      "volume = 100_000, all keys are unique, block cache disable, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache disable, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache enabled, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache enabled, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "ClockPro, volume = 100_000, all keys are unique, block cache enabled, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache disable, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache disable, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache enabled, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache enabled, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "ClockPro, volume = 100_000, all keys are unique, block cache enabled, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
	}

	sampleSize := 100_000

	for _, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			// Init a table
			sample := make([][]kvType, 0, tc.sstNum)
			for sst := 1; sst <= tc.sstNum; sst++ {
				fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
				assert.NoError(t, err)
				writer := go_sstable.NewWriter(
					fileWritable,
					common.TableV1,
					go_sstable.WithBlockRestartInterval(tc.restart),
					go_sstable.WithBlockSize(2*kB),
				)

				kvs := generateKV(sampleSize, tc.isUnique)
				sample = append(sample, kvs)
				for _, kv := range kvs {
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				}

				err = writer.Close()
				assert.NoError(t, err)
			}

			// Evaluate the result of the seek operations
			eg := errgroup.Group{}
			for sst := 1; sst <= tc.sstNum; sst++ {
				eg.Go(func() error {
					kvs := sample[sst-1]
					fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
					assert.NoError(t, err)
					var iterOpts []options.IteratorOptsFunc
					if tc.cacheSize > 0 {
						c := go_block_cache.NewMap(
							go_block_cache.WithMaxSize(int64(tc.cacheSize)),
							go_block_cache.WithCacheType(tc.cacheType),
							go_block_cache.WithShardNum(4),
						)
						iterOpts = []options.IteratorOptsFunc{
							options.WithBlockCache(c, fd),
						}
					}
					sharedBufferPool := predictable_size.NewPredictablePool()
					iter, err := go_sstable.NewSingularIterator(
						sharedBufferPool,
						fileReadable,
						iterOpts...,
					)
					require.NoError(t, err)

					defer func() {
						err := iter.Close()
						assert.NoError(t, err)
					}()

					firstKv := iter.First()
					assertKv(t, kvs[0], firstKv, 0)

					for i := 1; i < len(kvs); i++ {
						kv := iter.Next()
						assertKv(t, kvs[i], kv, i)
					}

					lastKv := iter.Next()
					assert.Nil(t, lastKv)

					return nil
				})
			}

			err := eg.Wait()
			assert.NoError(t, err)
		})
	}
}

func (w *SSTSuiteFS) Test_Iterator_First_Then_Next_Ops_MVCC_colblock() {
	type param struct {
		name      string
		restart   int
		isUnique  bool
		cacheSize int // 0 means no cache
		sstNum    int
		cacheType go_block_cache.CacheType
	}

	tests := []param{
		{
			name:      "volume = 100_000, all keys are unique, block cache disable, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache disable, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache enabled, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache enabled, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "ClockPro, volume = 100_000, all keys are unique, block cache enabled, single table",
			isUnique:  true,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, single table",
			isUnique:  false,
			restart:   5,
			sstNum:    1,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache disable, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache disable, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, all keys are unique, block cache enabled, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "volume = 100_000, keys are shared prefix, block cache enabled, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.LRU,
		},
		{
			name:      "ClockPro, volume = 100_000, all keys are unique, block cache enabled, multiple tables",
			isUnique:  true,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
		{
			name:      "ClockPro, volume = 100_000, keys are shared prefix, block cache enabled, multiple tables",
			isUnique:  false,
			restart:   5,
			sstNum:    3,
			cacheSize: 1 * mB,
			cacheType: go_block_cache.ClockPro,
		},
	}

	sampleSize := 100_000

	for _, tc := range tests {
		w.Run(tc.name, func() {
			t := w.T()
			// Init a table
			sample := make([][]kvType, 0, tc.sstNum)
			mvccComparer := NewMvccComparer()
			for sst := 1; sst <= tc.sstNum; sst++ {
				fileWritable, _, err := w.storage.Create(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
				assert.NoError(t, err)
				writer := go_sstable.NewWriter(
					fileWritable,
					common.TableV2,
					go_sstable.WithBlockRestartInterval(tc.restart),
					go_sstable.WithBlockSize(2*kB),
					go_sstable.WithComparer(mvccComparer),
				)

				kvs := generateKVWithSuffix(sampleSize, tc.isUnique)
				sample = append(sample, kvs)
				for _, kv := range kvs {
					err := writer.Set(kv.key, kv.value)
					assert.NoError(t, err, "failed to set")
				}

				err = writer.Close()
				assert.NoError(t, err)
			}

			// Evaluate the result of the seek operations
			eg := errgroup.Group{}
			for sst := 1; sst <= tc.sstNum; sst++ {
				eg.Go(func() error {
					kvs := sample[sst-1]
					fileReadable, fd, err := w.storage.Open(nogodb_common.TypeTable, nogodb_common.DiskfileNum(sst))
					assert.NoError(t, err)
					iterOpts := []options.IteratorOptsFunc{
						options.WithComparer(mvccComparer),
					}
					if tc.cacheSize > 0 {
						c := go_block_cache.NewMap(
							go_block_cache.WithMaxSize(int64(tc.cacheSize)),
							go_block_cache.WithCacheType(tc.cacheType),
							go_block_cache.WithShardNum(4),
						)
						iterOpts = append(iterOpts, []options.IteratorOptsFunc{
							options.WithBlockCache(c, fd),
						}...)
					}
					sharedBufferPool := predictable_size.NewPredictablePool()
					iter, err := go_sstable.NewSingularIterator(
						sharedBufferPool,
						fileReadable,
						iterOpts...,
					)
					require.NoError(t, err)

					defer func() {
						err := iter.Close()
						assert.NoError(t, err)
					}()

					firstKv := iter.First()
					assertKv(t, kvs[0], firstKv, 0)

					for i := 1; i < len(kvs); i++ {
						kv := iter.Next()
						assertKv(t, kvs[i], kv, i)
					}

					lastKv := iter.Next()
					assert.Nil(t, lastKv)

					return nil
				})
			}

			err := eg.Wait()
			assert.NoError(t, err)
		})
	}
}

func TestSSTSuiteFS(t *testing.T) {
	suite.Run(t, new(SSTSuiteFS))
}
