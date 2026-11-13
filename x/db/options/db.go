package options

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_block_cache "github.com/datnguyenzzz/nogodb/lib/go-block-cache"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"golang.org/x/sync/semaphore"
)

// Options holds the optional parameters for configuring nogodb, at db level
type DBOption struct {
	SST struct {
		Dir string
		// BytesPerSync syncs sstables periodically in order to smooth out writes to disk.
		// This option does not provide any persistency guarantee, but is used to avoid
		// latency spikes if the OS automatically decides to write out a large chunk
		// of dirty filesystem buffers.
		BytesPerSync int64 // Default: 512 KiB

		// BlockRestartInterval is the number of keys between restart points
		// for delta encoding of keys.
		BlockRestartInterval int // Default: 16

		// BlockSize is the target uncompressed size in bytes of each table block.
		BlockSize int // Default: 4096

		// BlockSizeThreshold finishes a block if the block size is larger than the
		// specified percentage of the target block size and adding the next entry
		// would cause the block to be larger than the target block size.
		BlockSizeThreshold int // Default: 90
	}

	// Cache is used to cache uncompressed blocks from sstables.
	Cache struct {
		Type nogodb_block_cache.CacheType
		Size int64
	}

	// LoadBlockSema, if set, is used to limit the number of blocks that can be
	// loaded (i.e. read from the filesystem) in parallel
	LoadBlockSema *semaphore.Weighted

	// Comparer defines a total ordering over the space of []byte keys: a 'less
	// than' relationship. The same comparison algorithm must be used for reads
	// and writes over the lifetime of the DB.
	Comparer nogodb_common.IComparer

	// FS provides the interface for persistent file storage.
	FS nogodb_fs.FS

	Manifest struct {
		Dir string
		// MaxManifestFileSize is the maximum size the MANIFEST file is allowed to
		// become. When the MANIFEST exceeds this size it is rolled over and a new
		// MANIFEST is created.
		MaxManifestFileSize int64
	}

	// MaxOpenFiles is a soft limit on the number of open files that can be
	// used by the DB.
	MaxOpenFiles int

	MemTable struct {
		// The size of a MemTable in steady state. The actual MemTable size starts at
		// min(256KB, MemTableSize) and doubles for each subsequent MemTable up to
		// MemTableSize. This reduces the memory pressure caused by MemTables for
		// short lived (test) DB instances.
		Size uint64 // The default value is 4MB.
	}

	WAL struct {
		Dir          string
		BytesPerSync int64 // Default: 256 KiB
	}

	Logger nogodb_common.Logger
}

func (o *DBOption) SetDefault() {
	if o.Cache.Type == nogodb_block_cache.Unknown {
		o.Cache.Type = nogodb_block_cache.LRU
	}

	if o.Cache.Size == 0 {
		o.Cache.Size = 8 << 20 // 8 MiB
	}

	if o.Comparer == nil {
		o.Comparer = nogodb_common.NewComparer()
	}

	if o.FS == nil {
		// TODO: At the moment only support Unix FS as a default
		o.FS = nogodb_fs.NewDefaultUnix()
	}

	if len(o.Manifest.Dir) == 0 {
		o.Manifest.Dir = "./nogodb/manifest"
	}

	if o.Manifest.MaxManifestFileSize == 0 {
		o.Manifest.MaxManifestFileSize = 128 << 20 // 128 MB
	}
	if o.MaxOpenFiles == 0 {
		o.MaxOpenFiles = 1000
	}
	if o.MemTable.Size <= 0 {
		o.MemTable.Size = 4 << 20 // 4 MB
	}

	if len(o.SST.Dir) == 0 {
		o.SST.Dir = "./nogodb/sst"
	}

	if o.SST.BytesPerSync == 0 {
		o.SST.BytesPerSync = 512 * 1024
	}

	if o.SST.BlockRestartInterval == 0 {
		o.SST.BlockRestartInterval = 16
	}

	if o.SST.BlockSize == 0 {
		o.SST.BlockSize = 2 << 12
	}

	if o.SST.BlockSizeThreshold == 0 {
		o.SST.BlockSizeThreshold = 90
	}

	if len(o.WAL.Dir) == 0 {
		o.WAL.Dir = "./nogodb/wal"
	}

	if o.WAL.BytesPerSync == 0 {
		o.WAL.BytesPerSync = 256 * 1024
	}

	if o.Logger == nil {
		o.Logger = nogodb_common.DefaultLogger
	}
}
