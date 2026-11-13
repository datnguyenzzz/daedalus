package compression

import (
	"encoding/binary"
	"fmt"

	"github.com/DataDog/zstd"
)

const (
	defaultLevel = 3
)

type zstdCompressor struct{}

func (z *zstdCompressor) GetType() CompressionType {
	return ZstdCompression
}

func (z *zstdCompressor) Compress(dst, src []byte) []byte {
	if len(dst) < binary.MaxVarintLen64 {
		dst = append(dst, make([]byte, binary.MaxVarintLen64-len(dst))...)
	}

	// Get the bound and allocate the proper amount of memory instead of relying on
	// Datadog/zstd to do it for us. This allows us to avoid memcopying data around
	// for the varIntLen prefix.
	bound := zstd.CompressBound(len(src))
	if cap(dst) < binary.MaxVarintLen64+bound {
		dst = make([]byte, binary.MaxVarintLen64, binary.MaxVarintLen64+bound)
	}

	zCtx := zstd.NewCtx()
	// Prefix with a uvarint encoding of len(src -- decompressed block)
	varIntLen := binary.PutUvarint(dst, uint64(len(src)))
	result, err := zCtx.CompressLevel(dst[varIntLen:varIntLen+bound], src, defaultLevel)
	if err != nil {
		panic("Error while compressing using Zstd.")
	}
	if &result[0] != &dst[varIntLen] {
		panic("Allocated a new buffer despite checking CompressBound.")
	}

	return dst[:varIntLen+len(result)]
}

func (z *zstdCompressor) Decompress(buf, compressed []byte) error {
	// The payload is prefixed with a varint encoding the length of
	// the decompressed block.
	_, prefixLen := binary.Uvarint(compressed)
	compressed = compressed[prefixLen:]
	if len(compressed) == 0 {
		return fmt.Errorf("decodeZstd: empty src buffer")
	}
	if len(buf) == 0 {
		return fmt.Errorf("decodeZstd: empty dst buffer")
	}
	zCtx := zstd.NewCtx()
	if _, err := zCtx.DecompressInto(buf, compressed); err != nil {
		return err
	}
	return nil
}

func (z *zstdCompressor) DecompressedLen(b []byte) (decompressedLen int, err error) {
	decodedLenU64, varIntLen := binary.Uvarint(b)
	if varIntLen <= 0 {
		return 0, fmt.Errorf("Decompressed size too small: %d", varIntLen)
	}
	return int(decodedLenU64), nil
}

var _ ICompression = (*zstdCompressor)(nil)
