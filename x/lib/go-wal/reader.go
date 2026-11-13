package go_wal

import (
	"bytes"
	"errors"
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_record "github.com/datnguyenzzz/nogodb/lib/common/record"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
)

type Offset struct {
	fileNum nogodb_common.DiskfileNum
	// offset indicates the file offset at which a record begins within
	// the fileNum
	offset int64
}

type WALReader struct {
	currReader *nogodb_record.Reader
	storager   nogodb_fs.Storage
	// off describes the current Offset within the WAL.
	off Offset
	// recordBuf is a buffer used to hold the latest record read from a physical
	// file, and then returned to the user
	//
	// This ensures we read its entirety so that Next() returns the next record,
	// even if the caller never exhausts the previous record's Reader.
	recordBuf bytes.Buffer
}

func NewWalReader(dir string, fs nogodb_fs.FS) (*WALReader, error) {
	storager, err := nogodb_fs.OpenVfsProvider(
		nogodb_fs.WithDirName(dir),
		nogodb_fs.WithFS(fs),
	)
	if err != nil {
		return nil, err
	}
	return &WALReader{
		storager: storager,
		off:      Offset{},
	}, nil
}

// Next returns a reader for the next record. It returns io.EOF if there
// are no more records. The reader returned becomes stale after the next Next
// call, and should no longer be used.
func (wr *WALReader) Next() (io.Reader, Offset, error) {
	var err error
	if wr.currReader == nil {
		if err = wr.nextFile(); err != nil {
			return nil, Offset{}, err
		}
	}

	for {
		wr.off.offset = wr.currReader.Offset()
		next, err := wr.currReader.Next()
		if errors.Is(err, io.EOF) {
			// the current file exhausted
			if err = wr.nextFile(); err != nil {
				return nil, Offset{}, err
			}

			continue
		}

		if err != nil {
			return nil, Offset{}, err
		}

		wr.recordBuf.Reset()
		_, err = io.Copy(&wr.recordBuf, next)
		if err != nil {
			return nil, Offset{}, err
		}

		// TODO(high): Parse Batch from recordBuf
		// and don't return record if SeqNum is <= last seen seq num

		return &wr.recordBuf, wr.off, nil
	}
}

func (wr *WALReader) nextFile() error {
	wr.currReader = nil
	wr.off.fileNum += 1
	wr.off.offset = 0

	rf, _, err := wr.storager.Open(nogodb_common.TypeWAL, wr.off.fileNum)
	if err != nil {
		return err
	}
	wr.currReader = nogodb_record.NewReader(rf)
	return nil
}

// Close the reader.
func (wr *WALReader) Close() error {
	wr.currReader = nil
	wr.recordBuf.Reset()
	return nil
}

var _ IWalReader = (*WALReader)(nil)
