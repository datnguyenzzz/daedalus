package uintcodex

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
)

type UintEncoder[T codex.UintType] struct {
	values []T
	rows   uint32
}

func (e *UintEncoder[T]) Init() {
	e.Reset()
}

// Reset reuses the existing encoder with its already allocated memory
func (e *UintEncoder[T]) Reset() {
	e.values = e.values[:0]
	e.rows = 0
}

func (e *UintEncoder[T]) Append(v T) {
	e.rows += 1
	block.GrowSize(&e.values, int(e.rows))

	e.values[e.rows-1] = v
}

// Size returns the size of the column, if the its row were encoded starting from an [offset]
func (e *UintEncoder[T]) Size(offset uint32) uint32 {
	minV, maxV := e.findMinMaxValue()
	reqB := byteWidth(maxV - minV)

	return 1 + 8 + e.rows*uint32(reqB)
}

func (e *UintEncoder[T]) DataType() codex.DataType {
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

// Finish serialises the encoded column into a [buf] from [offset], return the offset after written
func (e *UintEncoder[T]) Finish(rows, offset uint32, buf []byte) uint32 {
	if e.rows != uint32(len(e.values)) {
		// sense check
		panic(fmt.Sprintf("len of values: %d <> rows: %d", len(e.values), e.rows))
	}

	if rows < e.rows-1 || rows > e.rows {
		panic(fmt.Sprintf("UintEncoder only accepts to finish either all rows, or [all rows minus 1] target:%d >< total:%d", rows, e.rows))
	}

	e.values = e.values[:rows]

	minV, maxV := e.findMinMaxValue()
	reqB := byteWidth(maxV - minV)
	// start encoding and serialising to the [buf]
	// refer to the col_block/README.md for more detail about the layout
	buf[offset] = reqB // 1B
	offset += 1
	offset = serialise(buf, offset, 8, uint64(minV)) // 8B
	for _, v := range e.values {
		offset = serialise(buf, offset, reqB, v-minV)
	}

	// fmt.Println("Uint size", e.Size(offset), "enc.size()=", offset-before)
	return offset
}

// serialise puts the v into buf[offset:]
func serialise[T codex.UintType](buf []byte, offset uint32, width byte, v T) (nextOffset uint32) {
	switch width {
	case 0:
		// do not write any bytes
		nextOffset = offset
	case 1:
		buf[offset] = byte(v)
		nextOffset = offset + 1
	case 2:
		binary.LittleEndian.PutUint16(buf[offset:], uint16(v))
		nextOffset = offset + 2
	case 4:
		binary.LittleEndian.PutUint32(buf[offset:], uint32(v))
		nextOffset = offset + 4
	case 8:
		binary.LittleEndian.PutUint64(buf[offset:], uint64(v))
		nextOffset = offset + 8
	default:
		panic("UintEncoder tries serialise un-supported type")
	}

	return nextOffset
}

func (e *UintEncoder[T]) findMinMaxValue() (T, T) {
	maxV, minV := T(0), ^T(0)
	for _, v := range e.values {
		maxV = max(maxV, v)
		minV = min(minV, v)
	}

	return minV, maxV
}

// byteWidth returns maximum needed bytes to present a num
// only returns [0, 1, 2, 4, 8]
func byteWidth[T codex.UintType](num T) byte {
	bitWidth := bits.Len64(uint64(num))

	byteWidthTable := [65]uint8{
		// 0 bits => 0 bytes
		0,
		// 1..8 bits => 1 byte
		1, 1, 1, 1, 1, 1, 1, 1,
		// 9..16 bits => 2 bytes
		2, 2, 2, 2, 2, 2, 2, 2,
		// 17..32 bits => 4 bytes
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
		// 33..64 bits => 8 bytes
		8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
		8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	}

	return byteWidthTable[bitWidth]
}

var _ codex.IColumnEncoder[uint64] = (*UintEncoder[uint64])(nil)
