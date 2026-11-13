package block

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

type IIndexWriter interface {
	Add(key *nogodb_common.InternalKey, bh *common.BlockHandle) error
	// BuildIndex build the 2-level index for the SST, and write to the stable storage
	BuildIndex() (*common.BlockHandle, error)
	// ...
}
