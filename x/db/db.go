package db

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/datnguyenzzz/nogodb/db/options"
	"github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_block_cache "github.com/datnguyenzzz/nogodb/lib/go-block-cache"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	nogodb_wal "github.com/datnguyenzzz/nogodb/lib/go-wal"
)

// IReader is a readable key/value store.
//
// It is safe to call Get and NewIter from concurrent goroutines.
type IReader interface {
	// Get gets the value for the given key. It returns ErrNotFound if the DB
	// does not contain the key. On success, the caller MUST call closer.Close()
	// or a memory leak will occur.
	Get(key []byte) (value []byte, closer io.Closer, err error)

	// TODO(datnguyenzzz):
	//.  Enable when ready to support
	// NewIterWithContext() (*Iterator, error)

	// Close closes the Reader. It may or may not close any underlying io.Reader
	// or io.Writer, depending on how the DB was created.
	Close() error
}

// IWriter is a writable key/value store.
type IWriter interface {
	// TODO(datnguyenzzz):
	//.  Enable when ready to support
	// Apply the operations contained in the batch to the DB.
	// Apply(batch *Batch) error

	// Set sets the value for the given key. It overwrites any previous value
	// for that key.
	Set(key, value []byte) error

	// Delete deletes the value for the given key. Deletes are blind all will
	// succeed even if the given key does not exist.
	Delete(key []byte) error

	// TODO(datnguyenzzz):
	//. Enable when ready to support
	// Merge merges the value for the given key.
	// https://github.com/facebook/rocksdb/wiki/Merge-Operator
	// Merge(key, value []byte) error

	// TODO(datnguyenzzz):
	//.  Enable when ready to support
	// DeleteRange deletes all of the point keys (and values) in the range
	// [start,end) (inclusive on start, exclusive on end).
	// DeleteRange(start, end []byte) error

	// TODO(datnguyenzzz):
	//.  Enable when ready to support
	// RangeKeySet sets a range key mapping the key range [start, end) at the MVCC
	// timestamp suffix to value.
	// RangeKeySet(start, end, suffix, value []byte) error

	// TODO(datnguyenzzz):
	//.  Enable when ready to support
	// RangeKeyDelete deletes all of the range keys in the range [start,end)
	// RangeKeyDelete removes all range keys within the bounds, including those
	// with or without suffixes.
	// RangeKeyDelete(start, end []byte) error
}

// Internal iterfaces

type flushable interface {
	newFlushIter() nogodb_common.InternalIterator[common.InternalKV]
	// inuseBytes returns the number of inuse bytes by the flushable.
	inuseBytes() uint64
	// totalBytes returns the total number of bytes allocated by the flushable.
	totalBytes() uint64
	// readyForFlush returns true when the flushable is ready for flushing.
	readyForFlush() bool
}

// DB provides a concurrent, persistent ordered key/value store.
type DB struct {
	dirname  string
	opts     *options.DBOption
	dirLocks dirLock
	cmp      nogodb_common.IComparer

	// bgCtx is cancelled when the DB is closing. Background goroutines
	// (compactions, table stats loading) use this context
	bgCtx       context.Context
	bgCtxCancel context.CancelFunc
	closedCh    chan struct{}

	commit *commit

	sstStorager nogodb_fs.Storage

	mu struct {
		sync.Mutex
		versions *VersionSet
		log      struct { // Write ahead log
			writerManager nogodb_wal.IWalWriter
			writer        io.WriteCloser
		}
		mem struct { // Mem table
			mutable *memTable
			// Queue of flushables (the mutable memtable is at end)
			flushQueue []*memTable
		}
		compact struct { // Compactions
			// True when a flush is in progress.
			flushing bool
		}
	}

	cache nogodb_block_cache.IBlockCache

	// TODO. Write batch with index
	// https://github.com/facebook/rocksdb/wiki/Write-Batch-With-Index

	// TODO. commit pipeline.
	// The commit pipeline manages the steps in committing write batches,
	// such as writing the batch to the WAL and applying its contents to
	// the memtable

	// TODO. compaction scheduler
}

var (
	_ IReader = (*DB)(nil)
	_ IWriter = (*DB)(nil)
)

func Open(opt options.DBOption) (*DB, error) {
	var err error
	opt.SetDefault()
	db := &DB{
		closedCh: make(chan struct{}),
	}

	// TODO(high): reads the named database directory and recovers
	// the set of files encoding the database state at the moment
	// the previous process exited.
	// For now assume we open a new DB from a fresh state

	db.dirLocks, err = prepareDirs(opt)
	if err != nil {
		return nil, err
	}

	db.cache = nogodb_block_cache.NewMap(
		nogodb_block_cache.WithCacheType(opt.Cache.Type),
		nogodb_block_cache.WithMaxSize(int64(opt.Cache.Type)),
	)
	defer db.cache.Close()

	ctx, cancel := context.WithCancel(context.Background())
	db.bgCtx = ctx
	db.bgCtxCancel = cancel

	db.commit = &commit{
		nextSeqNum:    nogodb_common.SeqNum(db.mu.versions.GetLogSeqNum()),
		visibleSeqNum: nogodb_common.SeqNum(db.mu.versions.GetVisibleSeqNum()),
	}

	db.sstStorager, err = nogodb_fs.OpenVfsProvider(
		nogodb_fs.WithFS(opt.FS),
		nogodb_fs.WithDirName(opt.SST.Dir),
		nogodb_fs.WithBytesPerSync(opt.SST.BytesPerSync),
	)
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); db == nil {
			for _, mem := range db.mu.mem.flushQueue {
				mem.art = nil
			}

			if r != nil {
				panic(r)
			}
		}
	}()

	// Create a DB fresh version and create an initial manifest file
	// TODO(high): How to open from the recovered version
	db.mu.versions = &VersionSet{}
	if err = db.mu.versions.NewDB(&opt, &db.mu.Mutex); err != nil {
		return nil, err
	}

	walWriterManager, err := nogodb_wal.NewWalManager(
		opt.WAL.Dir,
		nogodb_wal.WithFS(opt.FS),
		nogodb_wal.WithBytesPerSync(uint32(opt.WAL.BytesPerSync)),
		nogodb_wal.WithLogger(opt.Logger),
	)
	if err != nil {
		return nil, err
	}
	defer walWriterManager.Close()
	db.mu.log.writerManager = walWriterManager

	db.mu.versions.SetVisibleSeqNum(db.mu.versions.GetLogSeqNum())

	newLogFileNum := db.mu.versions.GetNextFileNum()
	db.mu.log.writer, err = db.mu.log.writerManager.Create(newLogFileNum)
	if err != nil {
		return nil, err
	}

	db.mu.mem.mutable = newMemTable(
		*db.opts,
		nogodb_common.SeqNum(db.mu.versions.GetLogSeqNum()),
		newLogFileNum,
	)
	db.mu.mem.flushQueue = append(db.mu.mem.flushQueue, db.mu.mem.mutable)

	return db, nil
}

// maybeScheduleFlush schedules a flush if necessary.
// d.mu must be held when calling this.
func (d *DB) maybeScheduleFlush() {
	if d.mu.compact.flushing {
		return
	}

	if len(d.mu.mem.flushQueue) <= 1 {
		// the flushQueue is made by 1 mutable memTable
		// and ≥0 immutable memTables that are ready
		// for flushing
		return
	}

	if !d.readyForFlush() {
		return
	}

	d.mu.compact.flushing = true
	go d.flush()
}

// maybeScheduleCompaction schedules a compaction if necessary.
// d.mu must be held when calling this.
func (d *DB) maybeScheduleCompaction() {
	ctx := context.Background()
	d.mu.versions.AcquireLock(ctx)
	defer d.mu.versions.ReleaseLock(ctx)

	// what tables will be candidated for compaction ?
	// Picking level and tables by score
}

func (d *DB) readyForFlush() bool {
	var size uint64
	for i := 0; i < len(d.mu.mem.flushQueue)-1; i++ {
		if !d.mu.mem.flushQueue[i].readyForFlush() {
			break
		}
		size += d.mu.mem.mutable.totalBytes()
	}

	// Only flush once the sum of the queued memtable sizes exceeds half the
	// configured memtable size. This prevents flushing of memtables at startup
	// while we're undergoing the ramp period on the memtable size.
	minFlushSize := d.opts.MemTable.Size / 2
	return size > minFlushSize
}

type dirLock map[nogodb_common.ObjectType]*nogodb_fs.DirLock

// prepareDirs resolves the directory paths indicated and creates the
// directories if they don't exist, and acquires directory locks as necessary.
func prepareDirs(opt options.DBOption) (l dirLock, err error) {
	s := nogodb_fs.DirLockSet{}
	l = make(dirLock)

	mkdirThenLock := func(dir string) (*nogodb_fs.DirLock, error) {
		f, err := mkdirAll(dir, opt.FS)
		if err != nil {
			return nil, err
		}

		f.Close()

		l, err := s.Acquire(dir, opt.FS)
		if err != nil {
			return nil, err
		}

		return l, nil
	}

	for k := range nogodb_common.ObjectTypeToString {
		var lock *nogodb_fs.DirLock
		switch k {
		case nogodb_common.TypeTable:
			lock, err = mkdirThenLock(opt.SST.Dir)
			if err != nil {
				return nil, err
			}
		case nogodb_common.TypeWAL:
			lock, err = mkdirThenLock(opt.WAL.Dir)
			if err != nil {
				return nil, err
			}
		case nogodb_common.TypeManifest:
			lock, err = mkdirThenLock(opt.Manifest.Dir)
			if err != nil {
				return nil, err
			}
		default:
			continue
		}

		l[k] = lock
	}

	return l, nil
}

func mkdirAll(dir string, fs nogodb_fs.FS) (nogodb_fs.File, error) {
	var parentPaths []string

	for parentPath := fs.PathDir(dir); ; parentPath = fs.PathDir(parentPath) {
		parentPaths = append(parentPaths, parentPath)

		if fs.PathDir(parentPath) == parentPath {
			break
		}

		_, err := fs.Stat(parentPath)

		if err == nil {
			// closest existing ancestor
			break
		}

		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	for _, parentPath := range parentPaths {
		parentDir, err := fs.OpenDir(parentPath)
		if err != nil {
			return nil, err
		}

		err = parentDir.Sync()
		if err != nil {
			parentDir.Close()
			return nil, err
		}

		err = parentDir.Close()
		if err != nil {
			return nil, err
		}
	}

	return fs.OpenDir(dir)
}
