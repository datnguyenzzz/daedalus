package layoutcodex

import (
	"encoding/binary"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

type Header struct {
	version byte
	columns uint16 // number of columns
	rows    uint32 // number of rows
}

const (
	HeaderOffset = 7
)

func NewHeader(version common.TableVersion, cols uint16, rows uint32) *Header {
	return &Header{
		version: byte(version),
		columns: cols,
		rows:    rows,
	}
}

func (h *Header) Encode(offset uint32, buf []byte) uint32 {
	buf[offset] = h.version
	offset += 1
	binary.LittleEndian.PutUint16(buf[offset:], h.columns)
	offset += 2
	binary.LittleEndian.PutUint32(buf[offset:], h.rows)
	offset += 4

	return offset
}

func DecodeHeader(offset uint32, buf []byte) (*Header, uint32) {
	return &Header{
		version: buf[offset],
		columns: binary.LittleEndian.Uint16(buf[offset+1:]),
		rows:    binary.LittleEndian.Uint32(buf[offset+1+2:]),
	}, offset + HeaderOffset
}
