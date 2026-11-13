package compression

// CompressionType is the per-block compression algorithm to use.
type CompressionType int

// The available compression types.
const (
	NoCompression CompressionType = iota
	SnappyCompression
	ZstdCompression
)

type ICompression interface {
	GetType() CompressionType
	// Compress a block, appending the compressed data to dst[:0].
	Compress(dst, src []byte) []byte
	// Decompress decompresses compressed into buf. The buf slice must have the
	// exact size as the decompressed value. Callers may use DecompressedLen to
	// determine the correct size.
	Decompress(buf, compressed []byte) error
	// DecompressedLen returns the length of the provided block once decompressed,
	// allowing the caller to allocate a buffer exactly sized to the decompressed
	// payload.
	DecompressedLen(b []byte) (decompressedLen int, err error)
}

func NewCompressor(ct CompressionType) ICompression {
	switch ct {
	case SnappyCompression:
		return &snappyCompressor{}
	case ZstdCompression:
		return &zstdCompressor{}
	default:
		panic("unknown compression type")
	}
}
