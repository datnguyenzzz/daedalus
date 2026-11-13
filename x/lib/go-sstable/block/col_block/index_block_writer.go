package col_block

import (
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

var indexColumnsOrder = []string{
	"key",
	"offset",
	"length",
}

type IndexBlockWriter struct {
	// index key is the raw byte separator of 2 internal keys
	// prev_key ≤ index_key < current_key
	// The key is guarantee to be sorted in an increasing order
	keyEncoder rawbytescodex.RawByteEncoder

	blockHandleEncoder struct {
		offset uintcodex.UintEncoder[uint64]
		length uintcodex.UintEncoder[uint64]
	}

	layoutEncoder layoutcodex.LayoutEncoder
	rows          uint32
}

func (i *IndexBlockWriter) Reset() {
	i.keyEncoder.Reset()

	i.blockHandleEncoder.offset.Reset()
	i.blockHandleEncoder.length.Reset()

	i.rows = 0
}

func (i *IndexBlockWriter) Init() {
	i.keyEncoder.Init()

	i.blockHandleEncoder.offset.Init()
	i.blockHandleEncoder.length.Init()

	i.layoutEncoder.Reset()

	i.rows = 0
}

func (i *IndexBlockWriter) Rows() uint32 {
	return i.rows
}

// Add, caller ensure that the key is full UserKey
// that include MVCC suffix
func (i *IndexBlockWriter) Add(key []byte, bh *common.BlockHandle) {
	i.rows += 1

	// for index key, we only interested in the UserKey
	i.keyEncoder.Append(key)

	i.blockHandleEncoder.offset.Append(bh.Offset)
	i.blockHandleEncoder.length.Append(bh.Length)
}

func (i *IndexBlockWriter) Size() uint32 {
	offset := uint32(layoutcodex.HeaderOffset + layoutcodex.ColumnHeadSize*len(indexColumnsOrder))
	offset += i.keyEncoder.Size(offset)
	offset += i.blockHandleEncoder.offset.Size(offset)
	offset += i.blockHandleEncoder.length.Size(offset)
	offset += 1

	return offset
}

// Finish the writing to the current page, and prepare data for flushing to the storage
//
// Caller of the function must keep track of the current accumlated size of the block
// or using  DataBlockWriter.Size() function to get the size before finishing
func (i *IndexBlockWriter) Finish(rows uint32, size int) []byte {
	if rows < i.rows-1 || rows > i.rows {
		panic("IndexBlockWriter only accepts to finish either all rows, or [all rows minus 1]")
	}

	header := layoutcodex.NewHeader(common.TableV2, uint16(len(indexColumnsOrder)), rows)
	i.layoutEncoder.Init(size, header)

	for _, cName := range indexColumnsOrder {
		switch cName {
		case "key":
			i.layoutEncoder.Encode(rows, &i.keyEncoder)
		case "offset":
			i.layoutEncoder.Encode(rows, &i.blockHandleEncoder.offset)
		case "length":
			i.layoutEncoder.Encode(rows, &i.blockHandleEncoder.length)
		default:
			panic("IndexBlockWriter unhandled column name")
		}
	}

	return i.layoutEncoder.Data()
}

func NewIndexBlockWriter() *IndexBlockWriter {
	i := &IndexBlockWriter{
		keyEncoder:    rawbytescodex.RawByteEncoder{},
		layoutEncoder: layoutcodex.LayoutEncoder{},
	}

	i.blockHandleEncoder = struct {
		offset uintcodex.UintEncoder[uint64]
		length uintcodex.UintEncoder[uint64]
	}{
		offset: uintcodex.UintEncoder[uint64]{},
		length: uintcodex.UintEncoder[uint64]{},
	}

	i.Init()

	return i
}
