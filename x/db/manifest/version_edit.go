package manifest

import (
	"bytes"
	"encoding/binary"
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// Tags for the versionEdit disk format
const (
	tagComparator = iota
	tagNextFileNumber
)

// VersionEdit holds the state for an delta edit to a Version
type VersionEdit struct {
	ComparerName string

	// The next file number. A single counter is used to assign file numbers
	// for the WAL, MANIFEST, sstable files
	NextFileNum int64

	NewTables []NewTableEntry

	// MinUnflushedLogNum is the smallest WAL log file number corresponding to
	// mutations that have not been flushed to an sstable.
	MinUnflushedLogNum nogodb_common.DiskfileNum

	// LastSeqNum is an upper bound on the sequence numbers that have been
	// assigned in flushed WALs. Unflushed WALs (that will be replayed during
	// recovery) may contain sequence numbers greater than this value.
	LastSeqNum nogodb_common.SeqNum
}

type versionEditEncoder struct {
	*bytes.Buffer
}

func (e versionEditEncoder) writeString(s string) {
	e.writeUvarint(uint64(len(s)))
	e.WriteString(s)
}

func (e versionEditEncoder) writeUvarint(u uint64) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], u)
	e.Write(buf[:n])
}

func (ve *VersionEdit) Encode(w io.Writer) error {
	enc := versionEditEncoder{new(bytes.Buffer)}

	if len(ve.ComparerName) > 0 {
		enc.writeUvarint(tagComparator)
		enc.writeString(ve.ComparerName)
	}

	if ve.NextFileNum > 0 {
		enc.writeUvarint(tagNextFileNumber)
		enc.writeUvarint(uint64(ve.NextFileNum))
	}

	for _, table := range ve.NewTables {
		enc.writeUvarint(uint64(table.Level))
		enc.writeUvarint(uint64(table.Meta.TableNum))
		enc.writeUvarint(table.Meta.Size)
	}

	_, err := w.Write(enc.Bytes())
	return err
}
