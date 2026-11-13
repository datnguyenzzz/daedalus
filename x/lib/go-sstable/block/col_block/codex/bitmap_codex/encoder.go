package bitmapcodex

import (
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
)

type BitmapEncoder struct {
	// rows captures what is the maximum rows that the bitmap is holding
	// bitmap holds all number within range [0...rows-1]
	rows  uint32
	masks []uint64
	enc   uintcodex.UintEncoder[uint64]
}

func (b *BitmapEncoder) Init() {
	b.enc.Init()
	b.Reset()
}

// Reset reuses the existing encoder with its already allocated memory
func (b *BitmapEncoder) Reset() {
	b.rows = 0
	b.masks = b.masks[:0]
	b.enc.Reset()
}

func (b *BitmapEncoder) Append(v uint32) {
	i := v >> 6
	for len(b.masks) <= int(i) {
		b.masks = append(b.masks, 0)
	}

	b.masks[i] |= 1 << (v % 64)
	b.rows = max(b.rows, v+1)
}

// Size returns the size of the column, if the its row were encoded starting from an [offset]
func (b *BitmapEncoder) Size(offset uint32) uint32 {
	cnt := (b.rows + 63) >> 6 // [cnt] uint64 numbers for masks
	cnt_C := (cnt + 63) >> 6  // [cnt_C] uint64 numbers for index_masks
	return (cnt+cnt_C+1)<<3 + 1
}

func (b *BitmapEncoder) DataType() codex.DataType {
	return codex.BitmapDT
}

// Finish serialises the encoded column into a [buf] from [offset], return the offset after written
func (b *BitmapEncoder) Finish(rows, offset uint32, buf []byte) uint32 {
	maskCnt := (rows + 63) >> 6
	if len(b.masks) > int(maskCnt) {
		b.masks = b.masks[:maskCnt]
	} else {
		// fill the b.masks to have exact [masksCnt] masks
		for i := len(b.masks); i < int(maskCnt); i++ {
			b.masks = append(b.masks, 0)
		}
	}

	if len(b.masks) != int(maskCnt) {
		panic("len(b.masks) <> maskCnt")
	}

	// ensure all numbers beyond the [rows] are unset
	{
		m := rows % 64
		r := rows >> 6
		if r < uint32(len(b.masks)) {
			b.masks[r] &= uint64(1<<m) - 1
			for i := int(r + 1); i < len(b.masks); i++ {
				b.masks[i] = 0
			}
		}
	}

	for _, mask := range b.masks {
		b.enc.Append(mask)
	}

	// compute masks_index
	var checked bool
	maskIndexCnt := (maskCnt + 63) >> 6
	for i := range int(maskIndexCnt) {
		var index uint64
		for offset := i << 6; offset < min((i+1)<<6, len(b.masks)); offset++ {
			checked = checked || (offset == len(b.masks)-1)
			if b.masks[offset] == 0 {
				continue
			}

			index |= 1 << (offset - i<<6)
		}

		b.enc.Append(index)
	}

	if !checked {
		panic("miss indexing b.masks")
	}
	// fmt.Println("Bitmap size()=", b.Size(offset), "enc.size=", b.enc.Size(offset))

	return b.enc.Finish(uint32(len(b.masks))+maskIndexCnt, offset, buf)
}

var _ codex.IColumnEncoder[uint32] = (*BitmapEncoder)(nil)
