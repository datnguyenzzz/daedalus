package uintcodex

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
)

type UintDecoder[T codex.UintType] struct {
	// ptr points to the offset, where the UintColumn's values encoded to after [width][baseValue]
	// [width][baseValue]|[values...]
	//.                  |<-- ptr
	ptr       unsafe.Pointer
	rows      uint32
	width     byte
	baseValue uint64
}

func (u *UintDecoder[T]) Get(row uint32) T {
	if row >= u.rows {
		panic(fmt.Sprintf("outside of column block UintDecoder, %d >= %d", row, u.rows))
	}

	switch u.width {
	case 0:
		return T(u.baseValue)
	case 1:
		return T(u.baseValue) + T(*(*uint8)(unsafe.Add(u.ptr, uintptr(row))))
	case 2:
		return T(u.baseValue) + T(*(*uint16)(unsafe.Add(u.ptr, uintptr(row)<<1))) // 2 bytes per value
	case 4:
		return T(u.baseValue) + T(*(*uint32)(unsafe.Add(u.ptr, uintptr(row)<<2))) // 4 bytes per value
	case 8:
		return T(u.baseValue) + T(*(*uint64)(unsafe.Add(u.ptr, uintptr(row)<<3))) // 8 bytes per value
	default:
		panic("try decoding an UintDecoder but with a corrupted width")
	}
}

func (e *UintDecoder[T]) DataType() codex.DataType {
	var zero T
	switch any(zero).(type) {
	case uint8:
		return codex.Uint8DT
	case uint16:
		return codex.Uint16DT
	case uint32:
		return codex.Uint32DT
	case uint64:
		return codex.Uint64DT
	default:
		panic("try decoding an UintDecoder but with a corrupted width")
	}
}

// SeekGTE, by design, the RawBytesDecoder can't do Seek function.
// Therefore, this function only works only if caller ensure that
// RawBytesDecoder holds all keys in the sorted increasing order, from [from, to]
func (e *UintDecoder[T]) SeekGTE(key T, from, to int32) (rowIndex uint32, isEqual bool) {
	if from >= int32(e.rows) || to >= int32(e.rows) || from > to {
		panic("UintDecoder: searching range is out-bound")
	}
	if e.Get(uint32(from)) >= key {
		return 0, e.Get(uint32(from)) == key
	}

	if e.Get(uint32(to)) < key {
		return uint32(to) + 1, false
	}

	for from <= to {
		mid := (from + to) >> 1
		if e.Get(uint32(mid)) >= key {
			isEqual = e.Get(uint32(mid)) == key
			rowIndex = uint32(mid)
			to = mid - 1
		} else {
			from = mid + 1
		}
	}

	return rowIndex, isEqual
}

func (e *UintDecoder[T]) Slice(from, to uint32) T {
	panic("UintDecoder can not support Slice")
}

func (e *UintDecoder[T]) Rows() uint32 {
	return e.rows
}

// NewUintDecoder returns a UintDecoder with the offset of the next block
func NewUintDecoder[T codex.UintType](
	rows, offset uint32, buf []byte,
) (codex.IColumnDecoder[T], uint32) {
	// Refer to the col_block/README.md for more detail about the layout
	width := buf[offset]
	offset += 1
	baseValue := binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	return &UintDecoder[T]{
		ptr:       unsafe.Pointer(&buf[offset]),
		rows:      rows,
		width:     width,
		baseValue: baseValue,
	}, offset + rows*uint32(width)
}

var _ codex.IColumnDecoder[uint64] = (*UintDecoder[uint64])(nil)
