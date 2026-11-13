package rawbytescodex

import (
	"bytes"
	"fmt"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
)

type RawBytesDecoder struct {
	rows       uint32
	offsetsDec *uintcodex.UintDecoder[uint32]
	data       []byte
}

func (u *RawBytesDecoder) Get(row uint32) []byte {
	if row >= u.rows {
		panic("outside of column block RawBytesDecoder")
	}

	start := uint32(0)
	if row > 0 {
		start = u.offsetsDec.Get(row - 1)
	}

	end := u.offsetsDec.Get(row)

	return u.data[start:end]
}

func (u *RawBytesDecoder) Slice(from, to uint32) []byte {
	if from >= u.rows || to >= u.rows {
		panic(fmt.Sprintf("Slice [%d-%d] outside of column block RawBytesDecoder, %d", from, to, u.rows))
	}

	start := uint32(0)
	if from > 0 {
		start = u.offsetsDec.Get(from - 1)
	}

	end := u.offsetsDec.Get(to)

	return u.data[start:end]
}

func (e *RawBytesDecoder) DataType() codex.DataType {
	return codex.RawBytesDT
}

// SeekGTE, by design, the RawBytesDecoder can't do Seek function.
// Therefore, this function only works only if caller ensure that
// RawBytesDecoder holds all keys in the sorted increasing order, from [from, to]
func (e *RawBytesDecoder) SeekGTE(key []byte, from, to int32) (rowIndex uint32, isEqual bool) {
	if uint32(from) >= e.rows || uint32(to) >= e.rows || from > to {
		panic("RawBytesDecoder: searching range is out-bound")
	}
	if bytes.Compare(e.Get(uint32(from)), key) > 0 {
		return 0, false
	}

	if bytes.Compare(e.Get(uint32(to)), key) < 0 {
		return uint32(to) + 1, false
	}

	for from <= to {
		mid := (from + to) >> 1
		cp := bytes.Compare(e.Get(uint32(mid)), key)
		if cp >= 0 {
			isEqual = cp == 0
			rowIndex = uint32(mid)
			to = mid - 1
		} else {
			from = mid + 1
		}
	}

	return rowIndex, isEqual
}

func (e *RawBytesDecoder) Rows() uint32 {
	return e.rows
}

// NewRawBytesDecoder returns a RawBytesDecoder with the offset of the next block
func NewRawBytesDecoder(
	rows, offset uint32, data []byte,
) (codex.IColumnDecoder[[]byte], uint32) {
	iDec, offset := uintcodex.NewUintDecoder[uint32](rows, offset, data)
	dec, ok := iDec.(*uintcodex.UintDecoder[uint32])
	if !ok {
		panic("NewRawBytesDecoder failed to assert to uintcodex.UintDecoder[uint32]")
	}

	valuesLen := dec.Get(rows - 1)
	return &RawBytesDecoder{
		rows:       rows,
		offsetsDec: dec,
		data:       data[offset:],
	}, offset + valuesLen
}

var _ codex.IColumnDecoder[[]byte] = (*RawBytesDecoder)(nil)
