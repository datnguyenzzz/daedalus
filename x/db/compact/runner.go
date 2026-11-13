package compact

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	nogodb_sst "github.com/datnguyenzzz/nogodb/lib/go-sstable"
)

type Runner struct {
	bound nogodb_common.UserKeyBound
	iter  *Iter
}

// Result stores the result of a compaction - more specifically, the "data" part
// where we use the compaction iterator to write output tables.
type Result struct {
	Err    error
	Tables []struct {
		FileDesc   nogodb_fs.FileDesc
		LowSeqNum  nogodb_common.SeqNum
		HighSeqNum nogodb_common.SeqNum
	}
}

func NewRunner(iter *Iter, bound nogodb_common.UserKeyBound) *Runner {
	panic("")
}

func (r *Runner) HasMore() bool {
	panic("")
}

func (r *Runner) DoWrite(fd *nogodb_fs.FileDesc, tw nogodb_sst.IWriter) {
	panic("")
}

func (r *Runner) Finish() *Result {
	return nil
}
