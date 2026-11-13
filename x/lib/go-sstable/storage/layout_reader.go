package storage

import (
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
)

type layoutReader struct {
	fsReader go_fs.Readable
}

// ILayoutReader is used to perform reads that are related and might benefit from
// optimizations like read-ahead.
//
//go:generate mockery --name=ILayoutReader --case=underscore --disable-version-string
type ILayoutReader interface {
	// ReadAt reads len(p) bytes into p starting at offset off.
	//
	// Does not return partial results; if off + len(p) is past the end of the
	// object, an error is returned.
	ReadAt(p []byte, off uint64) error

	Close() error
}

// Implementations \\

func (l layoutReader) ReadAt(p []byte, off uint64) error {
	_, err := l.fsReader.ReadAt(p, int64(off))
	return err
}

func (l layoutReader) Close() error {
	return l.fsReader.Close()
}

func NewLayoutReader(fsReader go_fs.Readable) ILayoutReader {
	return &layoutReader{
		fsReader: fsReader,
	}
}

var _ ILayoutReader = (*layoutReader)(nil)
