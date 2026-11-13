package db

import (
	"github.com/datnguyenzzz/nogodb/db/manifest"
	"github.com/datnguyenzzz/nogodb/db/options"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// levelIter provides a merged view of the sstables in a level
type levelIter struct{}

func (l *levelIter) Close() error {
	panic("unimplemented")
}

func (l *levelIter) First() *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) IsClosed() bool {
	panic("unimplemented")
}

func (l *levelIter) Last() *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) Next() *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) Prev() *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) SeekGTE(key []byte) *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) SeekLTE(key []byte) *nogodb_common.InternalKV {
	panic("unimplemented")
}

func (l *levelIter) SeekPrefixGTE(prefix []byte, key []byte) *nogodb_common.InternalKV {
	panic("unimplemented")
}

func newLevelIter(
	opt *options.DBOption,
	tables *manifest.LevelIterator,
) *levelIter {
	panic("unimplemented")
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*levelIter)(nil)
