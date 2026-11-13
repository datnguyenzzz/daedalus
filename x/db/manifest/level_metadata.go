package manifest

import (
	"iter"

	// TODO(low): having nogodb_btree
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/google/btree"
)

const (
	degree = 16
)

// LevelMetadata contains metadata for all of the tables within a level of the LSM.
type levelMetadata struct {
	level int
	tree  *btree.BTreeG[TableMetadata]
}

func NewLevelMetadata(level int) *levelMetadata {
	return &levelMetadata{
		level: level,
		tree: btree.NewG(degree, func(a, b TableMetadata) bool {
			return a.Compare(b) <= 0
		}),
	}
}

// All returns an iterator over all tables in the level.
func (l *levelMetadata) All() iter.Seq[TableMetadata] {
	return func(yield func(TableMetadata) bool) {
		l.tree.Ascend(yield)
	}
}

func (l *levelMetadata) Clone() *levelMetadata {
	panic("")
}

func (l *levelMetadata) Insert(tm *TableMetadata) error {
	panic("")
}

func (l *levelMetadata) AggregateSize() uint64 {
	panic("")
}

func (l *levelMetadata) Iter(start, end int) *LevelIterator {
	panic("implement me")
}

// LevelIterator iterates over a set of tables' metadata within a range [start, end)
type LevelIterator struct {
	iter            btree.ItemIterator
	cmp             nogodb_common.IComparer
	tree            *btree.BTreeG[TableMetadata]
	start, end, pos int // -1 means at the boundary
}

func (l *LevelIterator) Curr() *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) Close() error {
	panic("unimplemented")
}

func (l *LevelIterator) First() *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) IsClosed() bool {
	panic("unimplemented")
}

func (l *LevelIterator) Last() *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) Next() *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) Prev() *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) SeekGTE(key []byte) *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) SeekLTE(key []byte) *TableMetadata {
	panic("unimplemented")
}

func (l *LevelIterator) SeekPrefixGTE(prefix []byte, key []byte) *TableMetadata {
	panic("unimplemented")
}

var _ nogodb_common.InternalIterator[TableMetadata] = (*LevelIterator)(nil)
