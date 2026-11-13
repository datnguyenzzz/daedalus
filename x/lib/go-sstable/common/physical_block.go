package common

import "encoding/binary"

const TrailerLen = 5

// PhysicalBlock represents a block  as it is stored
// physically on disk, including its trailer.
type PhysicalBlock struct {
	Data []byte
	// Trailer is the trailer at the end of a block, encoding the block type
	// (compression) and a checksum.
	Trailer [TrailerLen]byte
}

func (p *PhysicalBlock) SetData(data []byte) {
	p.Data = data
}

// Size of the physical block, includes its Trailer
func (p *PhysicalBlock) Size() uint64 {
	return uint64(len(p.Data)) + TrailerLen
}

func (p *PhysicalBlock) SetTrailer(auxiliary byte, checksum uint32) {
	var trailer [TrailerLen]byte
	trailer[0] = auxiliary
	binary.LittleEndian.PutUint32(trailer[1:], checksum)

	p.Trailer = trailer
}

const MaxBlockHandleBytes = 2 * binary.MaxVarintLen64

// BlockHandle is the file offset and length of a block.
type BlockHandle struct {
	// Offset identifies the offset of the block within the file.
	Offset uint64
	// Length is the length of the block data (INCLUDES the trailer).
	Length uint64
}

func (bh *BlockHandle) EncodeInto(buf []byte) int {
	n := binary.PutUvarint(buf, bh.Offset)
	m := binary.PutUvarint(buf[n:], bh.Length)
	return n + m
}

func (bh *BlockHandle) DecodeFrom(buf []byte) int {
	offset, n := binary.Uvarint(buf)
	if n <= 0 {
		return 0
	}
	length, m := binary.Uvarint(buf[n:])
	if m <= 0 {
		return 0
	}

	bh.Offset = offset
	bh.Length = length
	return n + m
}
