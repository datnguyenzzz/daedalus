package manifest

import nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"

// CompactionState is the compaction state of a file.
//
//	NotCompacting --> Compacting --> Compacted
//	      ^               |
//	      |               |
//	      +-------<-------+
type CompactionState uint8

// CompactionStates.
const (
	CompactionStateNotCompacting CompactionState = iota
	CompactionStateCompacting
	CompactionStateCompacted
)

// NewTableEntry holds the state for a new sstable or one moved from a different level.
type NewTableEntry struct {
	// level is the current level of the tableMeta
	Level int
	Meta  *TableMetadata
}

// TableMetadata is maintained for leveled sstables. TableMetadata does not
// contain the actual level of the sst, since such leveled-ssts can move across
// levels in different versions, while sharing the same TableMetadata.
type TableMetadata struct {
	// TableNum is the table number, unique across the lifetime of a DB.
	//
	TableNum nogodb_common.DiskfileNum
	// Size is the size of the file, in bytes.
	Size uint64

	CompactionState CompactionState

	// The lower and upper bounds SeqNums for the smallest and largest
	// sequence numbers in the table, across both point and range keys
	LowSeqNum  nogodb_common.SeqNum
	HighSeqNum nogodb_common.SeqNum
}

func (t *TableMetadata) Compare(t2 TableMetadata) int {
	panic("implement me")
}

func (t *TableMetadata) UserKeyBound() nogodb_common.UserKeyBound {
	panic("unimplemented")
}
