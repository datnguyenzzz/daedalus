package go_sstable

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/iterators"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
)

// Entry point for initializing every supported iterator

// NewSingularIterator returns an iterator for the singular keys in the SSTable
func NewSingularIterator(
	bpool *predictable_size.PredictablePool, // shared buffer pool across iterator
	r go_fs.Readable,
	optFuncs ...options.IteratorOptsFunc,
) (nogodb_common.InternalIterator[nogodb_common.InternalKV], error) {
	o := &options.IteratorOpts{
		Comparer: nogodb_common.NewComparer(),
	} // default is no cache

	for _, f := range optFuncs {
		f(o)
	}
	return iterators.NewIterator(bpool, r, o.Comparer, o)
}
