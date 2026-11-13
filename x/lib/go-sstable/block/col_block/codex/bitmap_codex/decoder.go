package bitmapcodex

import (
	"math/bits"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
)

type BitmapDecoder struct {
	rows uint32
	// indexOffset points to where the index masks sequence start at
	indexOffset uint32
	endOffset   uint32
	masks       *uintcodex.UintDecoder[uint64]
}

// return 0 if the [row] doesn't exists in the bitmap, 1 if it does
func (b *BitmapDecoder) Get(row uint32) uint32 {
	if (row >> 6) >= b.indexOffset {
		return 0
	}

	mask := b.masks.Get(row >> 6)
	return uint32(1 & (mask >> (row % 64)))
}

func (b *BitmapDecoder) Slice(from, to uint32) uint32 {
	panic("BitmapDecoder don't support Slice function")
}

func (b *BitmapDecoder) DataType() codex.DataType {
	return codex.BitmapDT
}

// SeekGTE finds the existing number in the bitmap that is ≥ key
// from-to is un-used
func (b *BitmapDecoder) SeekGTE(key uint32, from, to int32) (rowIndex uint32, isEqual bool) {
	idx := key >> 6
	if idx >= b.indexOffset {
		return b.rows, false
	}

	// for a reasonable dense bitmap, if the there's a
	// bit ≥ key set in the same word, return it.
	if nextBit := b.nextSetBitAt(b.masks.Get(idx), int(key%64)); nextBit < 64 {
		rowIndex = idx<<6 + uint32(nextBit)
		return rowIndex, nextBit == int(key%64)
	}

	// fast scanning by using index. The result is the least significant set bit
	// that is in the beyond buckets
	offset := b.indexOffset + idx>>6
	nextSet := b.nextSetBitAt(b.masks.Get(offset), int(idx%64+1))
	if nextSet == 64 {
		// move to the next index to and find the smallest set bit
		for {
			offset += 1
			if offset > b.endOffset {
				// not found
				return b.rows, false
			}

			if mask := b.masks.Get(offset); mask != 0 {
				nextSet = bits.TrailingZeros64(mask)
				break
			}
		}
	}

	// masks[idx] != 0
	idx = ((offset - b.indexOffset) << 6) + uint32(nextSet)
	rowIndex = (idx << 6) + uint32(bits.TrailingZeros64(b.masks.Get(idx)))

	return rowIndex, rowIndex == key
}

func (b *BitmapDecoder) Rows() uint32 {
	return b.rows
}

// nextSetBitAt finds the next set bit in the mask, where is ≥ [index]
// return 64 if is not found
func (b *BitmapDecoder) nextSetBitAt(mask uint64, index int) int {
	return bits.TrailingZeros64(mask &^ ((1 << index) - 1))
}

func NewBitmapDecoder(
	rows, offset uint32,
	data []byte,
) (codex.IColumnDecoder[uint32], uint32) {
	indexOffset := (rows + 63) >> 6
	dec := &BitmapDecoder{
		rows:        rows,
		indexOffset: indexOffset,
		endOffset:   indexOffset + (indexOffset+63)>>6 - 1,
	}

	maskRows := indexOffset + (indexOffset+63)>>6
	masks, offset := uintcodex.NewUintDecoder[uint64](maskRows, offset, data)

	if mDec, ok := masks.(*uintcodex.UintDecoder[uint64]); ok {
		dec.masks = mDec
	} else {
		panic("NewBitmapDecoder failed when assert to NewUintDecoder[uint64]")
	}

	return dec, offset
}

var _ codex.IColumnDecoder[uint32] = (*BitmapDecoder)(nil)
