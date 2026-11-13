package storage

import (
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

type layoutWriter struct {
	fsWritable go_fs.Writable
	// offset tracks the current offset of the entire physical table
	offset uint64
}

//go:generate mockery --name=ILayoutWriter --case=underscore --disable-version-string
type ILayoutWriter interface {
	// WritePhysicalBlock write a physicalBlock to the file system, and return BlockHandle
	// which contains the offset BEFORE writing, and data length
	WritePhysicalBlock(b common.PhysicalBlock) (common.BlockHandle, error)
	WriteRawBytes(b []byte) (common.BlockHandle, error)
	// Abort aborts writing the table, aborting the underlying writable too.
	Abort()
	Finish() error
}

var _ ILayoutWriter = (*layoutWriter)(nil)

// -- Implementations -- \\

func (w *layoutWriter) WritePhysicalBlock(pb common.PhysicalBlock) (common.BlockHandle, error) {
	if _, err := w.fsWritable.Write(pb.Data); err != nil {
		return common.BlockHandle{}, err
	}
	if _, err := w.fsWritable.Write(pb.Trailer[:]); err != nil {
		return common.BlockHandle{}, err
	}

	bh := common.BlockHandle{
		Offset: w.offset,
		Length: pb.Size(),
	}

	w.offset += pb.Size()

	return bh, nil
}

func (w *layoutWriter) WriteRawBytes(b []byte) (common.BlockHandle, error) {
	if _, err := w.fsWritable.Write(b); err != nil {
		return common.BlockHandle{}, err
	}

	bh := common.BlockHandle{
		Offset: w.offset,
		Length: uint64(len(b)),
	}

	w.offset += uint64(len(b))
	return bh, nil
}

func (w *layoutWriter) Abort() {
	w.fsWritable.Abort()
}

func (w *layoutWriter) Finish() error {
	return w.fsWritable.Finish()
}

func NewLayoutWriter(w go_fs.Writable) ILayoutWriter {
	return &layoutWriter{
		fsWritable: w,
	}
}
