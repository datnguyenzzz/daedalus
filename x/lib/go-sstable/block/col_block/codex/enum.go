package codex

type DataType byte

const (
	UnknownDT DataType = iota
	PrefixCompressedBytesDT
	RawBytesDT
	Uint8DT
	Uint16DT
	Uint32DT
	Uint64DT
	BitmapDT
)

type EncodableDataType interface {
	UintType | ByteType
}

type UintType interface {
	~uint8 | ~uint16 | ~uint32 | ~uint64
}

type ByteType interface {
	~[]byte
}

type IEncoderFinisher interface {
	DataType() DataType
	// Finish serialises the encoded column into a [buf] from [offset], return the offset after written
	Finish(row, offset uint32, buf []byte) uint32
}

type IColumnEncoder[T EncodableDataType] interface {
	Init()
	// Reset reuses the existing encoder with its already allocated memory
	Reset()
	Append(v T)
	// Size returns the size of the column, if the its row were encoded starting from an [offset]
	// TODO(high): Since we allow encoding [rows-1] rows, thus
	// it's better if we can estimate Size(rows, offset)
	Size(offset uint32) uint32

	IEncoderFinisher
}

type IColumnDecoder[T EncodableDataType] interface {
	Get(row uint32) T
	Slice(from, to uint32) T
	DataType() DataType
	SeekGTE(key T, from, to int32) (rowIndex uint32, isEqual bool)
	Rows() uint32
}

// DecoderInstructor create a decoder for a column that has "rows" rows
// and from data[offset:]
type DecoderInstructor[T EncodableDataType] func(rows, offset uint32, data []byte) (IColumnDecoder[T], uint32)
