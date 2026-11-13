package common

import "hash/crc32"

type ChecksumType byte

const (
	UnknownChecksum ChecksumType = iota
	CRC32Checksum
)

type checksumer struct {
	ct            ChecksumType
	auxiliaryByte [1]byte
}

type IChecksum interface {
	Checksum(block []byte, auxiliary byte) uint32
}

func (c checksumer) Checksum(block []byte, auxiliary byte) uint32 {
	var checksum uint32
	c.auxiliaryByte[0] = auxiliary
	switch c.ct {
	case CRC32Checksum:
		checksum = crc32.ChecksumIEEE(block)
		checksum = crc32.Update(checksum, crc32.IEEETable, c.auxiliaryByte[:])
	default:
		panic("unknown compression type")
	}

	return checksum
}

func NewChecksumer(ct ChecksumType) IChecksum {
	return &checksumer{
		ct: ct,
	}
}

var _ IChecksum = (*checksumer)(nil)
