package db

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/datnguyenzzz/nogodb/db/manifest"
	"github.com/datnguyenzzz/nogodb/db/options"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_record "github.com/datnguyenzzz/nogodb/lib/common/record"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"

	nogodb_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

// VersionSet manages a list of immutable versions, and manages the
// creation of a new version from the most recent version.
type VersionSet struct {
	dbOpt    *options.DBOption
	versions *manifest.VersionList
	// mu is the DB mutex lock
	mu *sync.Mutex

	// Next seqNum to use for WAL writes.
	logSeqNum nogodb_common.SeqNum

	// The upper bound on sequence numbers that have been assigned so far.
	// This is the sequence number for which records in the database are visible
	// during reads. The visible sequence number exists because committing a
	// batch (eg write the batch to the WAL and applying its contents to the memtable)
	// is an atomic operation, yet adding records to the memtable is done
	// without an exclusive lock (the skiplists used by both Pebble and RocksDB
	// are lock-free).
	visibleSeqNum nogodb_common.SeqNum

	// The next file number. A shared counter is used to assign file
	// numbers for the WAL, MANIFEST, sstable files.
	nextFileNum nogodb_common.DiskfileNum

	// minUnflushedLogNum is the smallest WAL log file number corresponding to
	// mutations that have NOT been flushed to an sstable.
	minUnflushedLogNum nogodb_common.DiskfileNum

	manifestWriter   *nogodb_record.Writer
	manifestStorager nogodb_fs.Storage

	lock nogodb_lock.ICtxLock

	cPicker *CompactionPicker
}

func (vs *VersionSet) init(
	dbOpt *options.DBOption,
	mu *sync.Mutex,
) (err error) {
	vs.dbOpt = dbOpt
	vs.mu = mu
	vs.versions.Init(mu)
	atomic.StoreUint64((*uint64)(&vs.logSeqNum), 0)
	atomic.StoreInt64((*int64)(&vs.nextFileNum), 1)
	vs.manifestStorager, err = nogodb_fs.OpenVfsProvider(
		nogodb_fs.WithDirName(dbOpt.Manifest.Dir),
		nogodb_fs.WithFS(dbOpt.FS),
	)
	vs.lock = nogodb_lock.NewLocalLock()
	if err != nil {
		return err
	}

	return nil
}

func (vs *VersionSet) NewDB(
	opt *options.DBOption,
	mu *sync.Mutex,
) error {
	if err := vs.init(opt, mu); err != nil {
		return err
	}

	blankVersion := manifest.NewVersion(opt.Comparer)
	vs.versions.PushBack(blankVersion)

	vs.cPicker = NewCompactionPicker(blankVersion)

	var err error
	if err = vs.createManifest(vs.GetNextFileNum()); err != nil {
		return err
	}
	if err = vs.manifestWriter.Flush(); err != nil {
		vs.dbOpt.Logger.Fatalf("manifest flushed failed: %v", err)
	}
	if err = vs.manifestStorager.Sync(nogodb_common.TypeManifest, vs.GetCurrentFileNum()); err != nil {
		vs.dbOpt.Logger.Fatalf("manifest sync failed: %v", err)
	}

	return nil
}

// UpdateVersion is not thread-safe, db.mu must be held. UpdateVersion first waits for any
// other version update to complete, releasing and reacquiring DB.mu.
func (vs *VersionSet) UpdateVersion(ve *manifest.VersionEdit) (err error) {
	ctx := context.Background()
	vs.lock.AcquireCtx(ctx)
	defer vs.lock.ReleaseCtx(ctx)

	if ve.MinUnflushedLogNum > 0 && vs.nextFileNum <= ve.MinUnflushedLogNum {
		panic(fmt.Sprintf("VersionSet: Detect inconsistent fileNum during UpdateVersion. NextFileNum: %d < ve.MinUnflushedLogNum: %d", vs.nextFileNum, ve.MinUnflushedLogNum))
	}

	ve.NextFileNum = int64(vs.nextFileNum)
	ve.LastSeqNum = nogodb_common.SeqNum(vs.GetLogSeqNum())

	currVer := vs.currentVersion()
	var newVersion *manifest.Version
	// Create new version
	// TODO(med): rotate the current manifest file is too large
	if err := func() error {
		vs.mu.Unlock()
		defer vs.mu.Lock()

		newVersion = &manifest.Version{
			Cmp: currVer.Cmp,
		}

		for i := range newVersion.Levels {
			newVersion.Levels[i] = currVer.Levels[i].Clone()

			if len(ve.NewTables) == 0 {
				continue
			}

			for _, tableEntry := range ve.NewTables {
				if tableEntry.Level != i {
					continue
				}

				newVersion.Levels[i].Insert(tableEntry.Meta)
			}
		}

		w, err := vs.manifestWriter.Next()
		if err != nil {
			return err
		}

		if err := ve.Encode(w); err != nil {
			return fmt.Errorf("failed encoding versionEdit. %w", err)
		}

		if err := vs.manifestWriter.Flush(); err != nil {
			return fmt.Errorf("failed flushing versionEdit. %w", err)
		}

		if err := vs.manifestStorager.Sync(nogodb_common.TypeManifest, vs.GetCurrentFileNum()); err != nil {
			return fmt.Errorf("failed Syncing versionEdit. %w", err)
		}

		return nil
	}(); err != nil {
		return err
	}

	vs.versions.PushBack(newVersion)
	vs.cPicker = NewCompactionPicker(newVersion)
	vs.minUnflushedLogNum = ve.MinUnflushedLogNum

	return nil
}

func (vs *VersionSet) createManifest(fileNum nogodb_common.DiskfileNum) error {
	var err error
	var manifestWriter *nogodb_record.Writer

	defer func() {
		if manifestWriter != nil {
			_ = manifestWriter.Close()
		}
		if err != nil {
			_ = vs.manifestStorager.Remove(nogodb_common.TypeManifest, fileNum)
		}
	}()

	var writable io.Writer
	writable, _, err = vs.manifestStorager.Create(nogodb_common.TypeManifest, fileNum)
	if err != nil {
		return err
	}
	manifestWriter = nogodb_record.NewWriter(writable)

	// Add all existing SSTables meta to the current version
	edit := &manifest.VersionEdit{
		ComparerName: vs.dbOpt.Comparer.Name(),
		NextFileNum:  int64(vs.nextFileNum),
	}

	for lvl, levelMeta := range vs.currentVersion().Levels {
		for tableMeta := range levelMeta.All() {
			edit.NewTables = append(edit.NewTables, manifest.NewTableEntry{
				Level: lvl,
				Meta:  &tableMeta,
			})
		}
	}

	w, err := manifestWriter.Next()
	if err != nil {
		return err
	}

	if err := edit.Encode(w); err != nil {
		return err
	}

	if vs.manifestWriter != nil {
		if err := vs.manifestWriter.Close(); err != nil {
			return err
		}

		vs.manifestWriter = nil
	}

	vs.manifestWriter, manifestWriter = manifestWriter, vs.manifestWriter

	return nil
}

func (vs *VersionSet) AcquireLock(ctx context.Context) {
	vs.AcquireLock(ctx)
}

func (vs *VersionSet) ReleaseLock(ctx context.Context) {
	vs.ReleaseLock(ctx)
}

func (vs *VersionSet) GetLogSeqNum() uint64 {
	return atomic.LoadUint64((*uint64)(&vs.logSeqNum))
}

func (vs *VersionSet) GetVisibleSeqNum() uint64 {
	return atomic.LoadUint64((*uint64)(&vs.visibleSeqNum))
}

func (vs *VersionSet) SetVisibleSeqNum(n uint64) {
	atomic.StoreUint64((*uint64)(&vs.visibleSeqNum), n)
}

func (vs *VersionSet) currentVersion() *manifest.Version {
	return vs.versions.Back()
}

func (vs *VersionSet) GetNextFileNum() nogodb_common.DiskfileNum {
	return nogodb_common.DiskfileNum(atomic.AddInt64((*int64)(&vs.nextFileNum), 1) - 1)
}

func (vs *VersionSet) GetCurrentFileNum() nogodb_common.DiskfileNum {
	return nogodb_common.DiskfileNum(atomic.LoadInt64((*int64)(&vs.nextFileNum)) - 1)
}
