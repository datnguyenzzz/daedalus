package block

import (
	"encoding/binary"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

// CompressToPb compress the raw bytes by using the given compressor
func CompressToPb(
	compressor compression.ICompression,
	checksumer nogodb_common.IChecksum,
	rawData []byte,
) *common.PhysicalBlock {
	pb := &common.PhysicalBlock{}
	compressed := compressor.Compress(nil, rawData)
	checksum := checksumer.Checksum(compressed, byte(compressor.GetType()))
	pb.SetData(compressed)
	pb.SetTrailer(byte(compressor.GetType()), checksum)
	return pb
}

func CommonPrefix(a, b []byte) int {
	var shared int
	compare8Byte := func(idx int) bool {
		va := binary.LittleEndian.Uint64(a[idx:])
		vb := binary.LittleEndian.Uint64(b[idx:])
		return va == vb
	}
	for ; shared+8 < min(len(a), len(b)); shared += 8 {
		// Iterate 8 bytes at once
		if !compare8Byte(shared) {
			break
		}
	}

	for ; shared < min(len(a), len(b)); shared++ {
		if a[shared] != b[shared] {
			break
		}
	}

	return shared
}

// GrowSize inlinely grows size of the buf to expectedSize
func GrowSize[T any](buf *[]T, expectedSize int) {
	if cap(*buf) < expectedSize {
		newCap := max(32, cap(*buf)<<1)
		for newCap <= expectedSize {
			if newCap <= 1024 {
				newCap <<= 1
			} else {
				newCap += newCap / 4
			}
		}

		nb := make([]T, expectedSize, newCap)
		copy(nb[:len(*buf)], *buf)
		*buf = nb
	} else {
		*buf = (*buf)[:expectedSize]
	}
}
