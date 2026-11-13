package rawbytescodex

import (
	"fmt"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	unit_codex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
)

type RawByteEncoder struct {
	values []byte
	rows   uint32
	// len of values should be fit in 32-bits
	offsets    unit_codex.UintEncoder[uint32]
	lastOffset int
}

func (r *RawByteEncoder) Init() {
	r.offsets.Init()
	r.Reset()
}

// Reset reuses the existing encoder with its already allocated memory
func (r *RawByteEncoder) Reset() {
	r.rows = 0
	r.values = r.values[:0]
	r.offsets.Reset()
}

func (r *RawByteEncoder) Append(v []byte) {
	if uint64(len(r.values))+uint64(len(v)) > uint64(^uint32(0)) {
		panic("RawByteEncoder values becomes too large")
	}
	r.lastOffset = len(r.values)
	r.rows++
	r.values = append(r.values, v...)
	r.offsets.Append(uint32(len(r.values)))
}

func (e *RawByteEncoder) DataType() codex.DataType {
	return codex.RawBytesDT
}

// Size returns the size of the column, if the its row were encoded starting from an [offset]
func (r *RawByteEncoder) Size(offset uint32) uint32 {
	return uint32(len(r.values)) + r.offsets.Size(offset)
}

// Finish serialises the encoded column into a [buf] from [offset], return the offset after written
func (r *RawByteEncoder) Finish(rows, offset uint32, buf []byte) uint32 {
	if rows < r.rows-1 || rows > r.rows {
		panic(fmt.Sprintf("RawByteEncoder only accepts to finish either all rows, or [all rows minus 1] %d >< %d", rows, r.rows))
	}

	offset = r.offsets.Finish(rows, offset, buf)

	end := len(r.values)
	if rows == r.rows-1 {
		end = r.lastOffset
	}
	r.values = r.values[:end]

	copy(buf[offset:], r.values)

	// fmt.Println("RawByteEncoder, size()=", r.Size(offset), "enc.size()=", offset-before+uint32(len(r.values)))
	return offset + uint32(len(r.values))
}

var _ codex.IColumnEncoder[[]byte] = (*RawByteEncoder)(nil)
