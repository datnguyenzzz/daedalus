package compression

import (
	"fmt"

	"github.com/golang/snappy"
)

type snappyCompressor struct{}

func (s *snappyCompressor) GetType() CompressionType {
	return SnappyCompression
}

func (s *snappyCompressor) Compress(dst, src []byte) []byte {
	dst = dst[:cap(dst):cap(dst)]
	return snappy.Encode(dst, src)
}

func (s *snappyCompressor) Decompress(buf, compressed []byte) error {
	res, err := snappy.Decode(buf, compressed)
	if err != nil {
		return err
	}
	if len(res) != len(buf) || (len(res) > 0 && &res[0] != &buf[0]) {
		return fmt.Errorf("snappy: compressed data mismatch")
	}
	return nil
}

func (s *snappyCompressor) DecompressedLen(b []byte) (decompressedLen int, err error) {
	return snappy.DecodedLen(b)
}

var _ ICompression = (*snappyCompressor)(nil)
