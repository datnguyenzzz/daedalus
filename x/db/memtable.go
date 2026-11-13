package db

import (
	"context"

	"github.com/datnguyenzzz/nogodb/db/options"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_art "github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree"
)

// memTable implements an in-memory layer of the LSM. A memTable is mutable,
// but append-only. Records are added, but never removed. Deletion is supported
// via tombstones, but it is up to higher level code to support processing those
// tombstones.
type memTable struct {
	cmp nogodb_common.IComparer
	// Channel which is closed when the flushable has been flushed.
	flushed chan struct{}
	art     nogodb_art.ITree[[]byte]

	// The current SeqNum at the time the memtable was created. This is
	// guaranteed to be less than or equal to any seqnum stored in the
	// memtable. Can be represented as a smallest SeqNum of all keys that
	// are stored in the memTable
	seqNum nogodb_common.SeqNum

	// logFileNum corresponds to the corresponsding WAL file to this
	// memtable
	logFileNum nogodb_common.DiskfileNum
}

func newMemTable(
	opt options.DBOption,
	segNum nogodb_common.SeqNum,
	logFileNum nogodb_common.DiskfileNum,
) *memTable {
	m := &memTable{
		cmp:        opt.Comparer,
		seqNum:     segNum,
		logFileNum: logFileNum,
		art:        nogodb_art.NewTree[[]byte](context.Background()),
	}

	return m
}

// Prepare reserves space for the batch in the memtable and references the
// memtable preventing it from being flushed until the batch is applied. Note
// that prepare is not thread-safe, while apply is. The caller must call
// writerUnref() after the batch has been applied.
func (m *memTable) prepare(b *Batch) {
	panic("implement me!")
}

// apply applies the mutations in the batch to the memtable
func (m *memTable) apply(b *Batch, seqNum nogodb_common.SeqNum) error {
	panic("implement me!")
}

// writerUnref drops a ref on the memtable. Returns true if this was
// the last ref.
func (m *memTable) writerUnref() (wasLastRef bool) {
	panic("implement me")
}

// Flush to L0

func (m *memTable) newFlushIter() nogodb_common.InternalIterator[nogodb_common.InternalKV] {
	return nil
}

// inuseBytes returns the number of inuse bytes by the flushable.
func (m *memTable) inuseBytes() uint64 { return 0 }

// totalBytes returns the total number of bytes allocated by the flushable.
func (m *memTable) totalBytes() uint64 { return 0 }

// readyForFlush returns true when the flushable is ready for flushing.
func (m *memTable) readyForFlush() bool { return false }

var _ flushable = (*memTable)(nil)
