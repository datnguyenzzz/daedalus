package db

import (
	"context"
	"runtime/pprof"
	"slices"

	"github.com/datnguyenzzz/nogodb/db/compact"
	"github.com/datnguyenzzz/nogodb/db/manifest"
	"github.com/datnguyenzzz/nogodb/db/options"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	nogodb_pool "github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	nogodb_sst "github.com/datnguyenzzz/nogodb/lib/go-sstable"
	sst_common "github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

var flushLabels = pprof.Labels("flush", "to-level", "L0")

type compaction struct {
	cmp        nogodb_common.IComparer
	pool       nogodb_pool.PredictablePool
	logger     nogodb_common.Logger
	flushList  []flushable
	bound      nogodb_common.UserKeyBound
	startLevel *compactionLevel // TODO: See compaction picker, support multiple levels in 1 compaction job
	outLevel   *compactionLevel
}

type compactionLevel struct {
	level  int
	tables manifest.LevelIterator
}

// newCompaction compacts input tables to the next level
func newCompaction() *compaction {
	panic("implement me!!!")
}

// newFlush flushs memtables to SST L0.
func newFlush(o *options.DBOption, memTables []*memTable) *compaction {
	flushList := make([]flushable, 0, len(memTables))
	for _, t := range memTables {
		flushList = append(flushList, t)
	}
	c := &compaction{
		cmp:        o.Comparer,
		pool:       *nogodb_pool.NewPredictablePool(),
		logger:     o.Logger,
		startLevel: &compactionLevel{level: -1},
		outLevel:   &compactionLevel{level: 0},
		flushList:  flushList,
	}

	updatePointBound := func(iter nogodb_common.InternalIterator[nogodb_common.InternalKV]) {
		if kv := iter.First(); kv != nil {
			if c.bound.Start == nil || c.cmp.Compare(c.bound.Start, kv.K.UserKey) > 0 {
				c.bound.Start = slices.Clone(kv.K.UserKey)
			}
		}

		if kv := iter.Last(); kv != nil {
			if c.bound.End.Key == nil || c.cmp.Compare(c.bound.End.Key, kv.K.UserKey) < 0 {
				c.bound.End.Key = slices.Clone(kv.K.UserKey)
				c.bound.End.Kind = nogodb_common.Inclusive
			}
		}
	}

	for _, flush := range c.flushList {
		updatePointBound(flush.newFlushIter())
	}

	return c
}

func (d *DB) flush() {
	pprof.Do(context.Background(), flushLabels, func(ctx context.Context) {
		d.mu.Lock()
		defer d.mu.Unlock()

		if err := d.__flush(); err != nil {
			// uhmm what do to if flushing failed :thinking:
		}

		// More flush work may have arrived while we were flushing, so schedule
		// another flush if needed.
		d.maybeScheduleFlush()

		// The flush may have produced too many files in L0, so schedule a
		// compaction if needed.
		d.maybeScheduleCompaction()
	})
}

// __flush runs a compaction that copies the immutable memtables
// from memory to disk.
// Note: Must call this function with db.mu.Lock held
func (d *DB) __flush() (err error) {
	var n int
	for ; n < len(d.mu.mem.flushQueue)-1; n++ {
		if !d.mu.mem.flushQueue[n].readyForFlush() {
			break
		}
	}

	if n == 0 {
		return nil
	}

	// Require that every memtable being flushed has a log number less than the
	// new minimum unflushed log number
	minUnflushedLogFileNum := d.mu.mem.flushQueue[n].logFileNum
	for i := 0; i < n; i++ {
		if logNum := d.mu.mem.flushQueue[i].logFileNum; logNum >= minUnflushedLogFileNum {
			panic("flush: There is a memtable that has logFileNum > minUnflushedLogFileNum")
		}
	}

	c := newFlush(d.opts, d.mu.mem.flushQueue[:n])
	var ve *manifest.VersionEdit
	ve, err = d.runCompaction(c)
	if err != nil {
		return err
	}
	ve.MinUnflushedLogNum = minUnflushedLogFileNum

	// update new version
	err = d.mu.versions.UpdateVersion(ve)
	if err != nil {
		return err
	}

	for _, mt := range d.mu.mem.flushQueue[:n] {
		close(mt.flushed)
	}

	d.mu.mem.flushQueue = d.mu.mem.flushQueue[n:]
	return nil
}

func (d *DB) runCompaction(c *compaction) (ve *manifest.VersionEdit, err error) {
	// release the db.mu.Lock while doing I/O
	d.mu.Unlock()
	defer d.mu.Lock()

	// Note: Compactions shoud avoids polluting the block cache with
	// blocks that won't likely be read again

	var iters []nogodb_common.InternalIterator[nogodb_common.InternalKV]

	if len(c.flushList) > 0 {
		// flush from Memtables to L0
		iters = make([]nogodb_common.InternalIterator[nogodb_common.InternalKV], 0, len(c.flushList))
	} else {
		// compact Li tables to Li+1
		iters = append(iters, newLevelIter(d.opts, &c.startLevel.tables))
	}
	// TODO: support Range Queries (list, delete) iteration

	defer func() {
		if err != nil {
			for _, iter := range iters {
				if iter == nil {
					continue
				}

				_ = iter.Close()
			}
		}
	}()

	for _, flush := range c.flushList {
		iters = append(iters, flush.newFlushIter())
	}

	// TODO: Support merging all iters into one single iteration
	dfns := make([]nogodb_common.DiskfileNum, 16)
	results := make([]*compact.Result, 16)

	for _, iter := range iters {
		cIter := compact.NewIter(c.cmp, iter)
		cRunner := compact.NewRunner(cIter, c.bound)
		dfns = dfns[:0]

		for cRunner.HasMore() {
			dfn := d.mu.versions.GetNextFileNum()
			dfns = append(dfns, dfn)
			writable, fd, err := d.sstStorager.Create(nogodb_common.TypeTable, dfn)
			if err != nil {
				return nil, err
			}

			opts := []nogodb_sst.WriteOptFn{
				nogodb_sst.WithComparer(d.opts.Comparer),
				nogodb_sst.WithBlockRestartInterval(d.opts.SST.BlockRestartInterval),
				nogodb_sst.WithBlockSize(d.opts.SST.BlockSize),
				nogodb_sst.WithBlockSizeThreshold(float32(d.opts.SST.BlockSizeThreshold) / 100.0),
				nogodb_sst.WithCompression(compression.SnappyCompression),
			}
			writer := nogodb_sst.NewWriter(writable, sst_common.TableV2, opts...)

			cRunner.DoWrite(&fd, writer)
		}
		res := cRunner.Finish()
		if res == nil {
			panic("cRunner.Finish() should not return nil")
		}
		if res.Err == nil {
			for _, dfn := range dfns {
				_ = d.sstStorager.Sync(nogodb_common.TypeTable, dfn)
			}
		}

		results = append(results, res)
	}

	ve = makeVersionEdit(c, results)

	for _, res := range results {
		if res.Err == nil {
			continue
		}

		// TODO(high): Delete any created zombie tables, aka the tables
		// are created (with iterator is still accessible) but got any
		// error
	}

	return ve, nil
}

func makeVersionEdit(c *compaction, results []*compact.Result) *manifest.VersionEdit {
	ve := &manifest.VersionEdit{}

	for _, res := range results {
		if res.Err != nil {
			continue
		}

		for _, table := range res.Tables {
			entry := &manifest.NewTableEntry{
				Level: c.outLevel.level,
				Meta: &manifest.TableMetadata{
					TableNum:   table.FileDesc.Num,
					LowSeqNum:  table.LowSeqNum,
					HighSeqNum: table.HighSeqNum,
				},
			}
			ve.NewTables = append(ve.NewTables, *entry)
		}
	}

	return ve
}
