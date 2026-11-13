package go_wal

import (
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

type IWalWriter interface {
	// List returns the WALs info in ascending order of file name.
	// List does not perform I/O
	List() []nogodb_common.DiskfileNum
	// Obsolete informs the manager that all WALs less than minUnflushedNum are obsolete.
	Obsolete(minUnflushedNum nogodb_common.DiskfileNum) (toDelete []nogodb_common.DiskfileNum, err error)
	// Create creates a new WAL. NumWALs passed to successive Create calls must be
	// monotonically increasing, and be greater than any NumWAL seen earlier. The
	// caller must close the previous Writer before calling Create.
	Create(fileNum nogodb_common.DiskfileNum) (io.WriteCloser, error)
	Close() error
}

type IWalReader interface {
	// Next returns a reader for the next record. It returns io.EOF if there
	// are no more records. The reader returned becomes stale after the next Next
	// call, and should no longer be used.
	Next() (io.Reader, Offset, error)
	// Close the reader.
	Close() error
}
