package layoutcodex

import (
	"encoding/binary"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
)

const (
	maxBlockRetainedSize = 256 << 10
	// 1B - data type, 4B - offset to the column
	ColumnHeadSize = 5
)

type LayoutEncoder struct {
	buf          []byte
	headerOffset uint32
	pageOffset   uint32
}

// Init caller must calculate and pass the total needed size after encoding
func (p *LayoutEncoder) Init(size int, h *Header) {
	block.GrowSize(&p.buf, size)

	p.headerOffset = h.Encode(0, p.buf)
	// refer to the README to understand the page layout
	p.pageOffset = p.headerOffset + uint32(h.columns)*ColumnHeadSize
}

func (p *LayoutEncoder) Reset() {
	if cap(p.buf) >= maxBlockRetainedSize {
		p.buf = nil
	}

	p.headerOffset = 0
}

// Encode finishes the given column encoder into the row-th
// refer to the README to understand the layout
func (p *LayoutEncoder) Encode(row uint32, enc codex.IEncoderFinisher) {
	p.buf[p.headerOffset] = byte(enc.DataType())
	binary.LittleEndian.PutUint32(p.buf[p.headerOffset+1:], p.pageOffset)
	p.headerOffset += ColumnHeadSize

	p.pageOffset = enc.Finish(row, p.pageOffset, p.buf)
	// fmt.Println("LayoutEncoder", "size=", p.pageOffset-before, "pageOffset=", p.pageOffset, "buf=", len(p.buf))
}

func (p *LayoutEncoder) Data() []byte {
	p.buf[p.pageOffset] = 0x00 // padding unused byte
	return p.buf
}
