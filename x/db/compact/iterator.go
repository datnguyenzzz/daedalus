package compact

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// Iter provides a forward-only iterator that encapsulates the logic for
// collapsing entries during compaction. The high-level structure for
// compact.Iter is to iterate over its internal iterator and output 1 entry
// for every user-key.
// In the future, this Iter will have to handle some complications
//  1. Omit redudant DEL: Such as the entries a.DEL.2 and a.PUT.1
//  2. Merges
//  3. Range deletion
type Iter struct {
	cmp   nogodb_common.IComparer
	iters nogodb_common.InternalIterator[nogodb_common.InternalKV]
	kv    *nogodb_common.InternalKV
}

func NewIter(
	cmp nogodb_common.IComparer,
	iter nogodb_common.InternalIterator[nogodb_common.InternalKV],
) *Iter {
	panic("")
}
