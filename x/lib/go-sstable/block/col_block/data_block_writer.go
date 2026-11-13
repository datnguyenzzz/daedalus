package col_block

import (
	"fmt"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	bitmapcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/bitmap_codex"
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	prefixbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/prefix_bytes_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
)

const (
	defaultBundleSize = 16
)

var columnsOrder = []string{
	"prefix",
	"suffix",
	"trailer",
	"values",
	"prefixChangedAt",
}

type DataBlockWriter struct {
	comparer nogodb_common.IComparer

	keyEncoder struct {
		// prefix stores the prefix of an user key, aka an actual user key
		prefix prefixbytescodex.PrefixBytesEncoder
		// suffix stores the MVCC index of the key
		suffix rawbytescodex.RawByteEncoder
	}

	columnEncoder struct {
		// trailer stores internalKey.Trailer
		trailer uintcodex.UintEncoder[uint64]
		// values stores values
		values rawbytescodex.RawByteEncoder
		// prefixChanged keeps track when a new key prefix begin
		// For example:
		//   aaabbb|c  < 0
		//   aaabbb|d
		//   aaabbb|e
		//   aaabb|c   < 3
		//   aaa|c     < 4
		//   aaa|e
		//   a|d       < 6
		// Why:
		//   We need it for seeking the key. After finding the i-th prefix that equals
		//   the target key, we need to seek the suffix in [prefixChange[i-1], prefixChange[i]-1]
		//   because the keys are in increasing order, which ensures that the suffix in that range is sorted.
		prefixChangedAt bitmapcodex.BitmapEncoder
	}

	layoutEncoder layoutcodex.LayoutEncoder

	currKey *nogodb_common.InternalKey
	rows    uint32

	lastPrefix []byte
}

func (d *DataBlockWriter) Reset() {
	d.keyEncoder.prefix.Reset()
	d.keyEncoder.suffix.Reset()

	d.columnEncoder.trailer.Reset()
	d.columnEncoder.values.Reset()
	d.columnEncoder.prefixChangedAt.Reset()

	d.layoutEncoder.Reset()
	d.rows = 0
	d.lastPrefix = nil
	d.currKey = nil
}

// Add adds a key-value pair to the sstable.
func (d *DataBlockWriter) Add(key nogodb_common.InternalKey, value []byte) {
	d.rows += 1
	d.currKey = &key

	prefixLen := d.comparer.Split(key.UserKey)
	d.keyEncoder.prefix.Append(key.UserKey[:prefixLen])
	d.keyEncoder.suffix.Append(key.UserKey[prefixLen:])

	if len(d.lastPrefix) == 0 || d.comparer.Compare(key.UserKey[:prefixLen], d.lastPrefix) != 0 {
		d.lastPrefix = key.UserKey[:prefixLen]
		d.columnEncoder.prefixChangedAt.Append(d.rows - 1)
		// fmt.Println(d.rows - 1)
	}

	d.columnEncoder.trailer.Append(uint64(key.Trailer))

	d.columnEncoder.values.Append(value)
}

func (d *DataBlockWriter) Rows() uint32 {
	return d.rows
}

func (d *DataBlockWriter) CurrKey() *nogodb_common.InternalKey {
	return d.currKey
}

func (d *DataBlockWriter) Size() uint32 {
	// refer to the README to understand the layout
	offset := uint32(layoutcodex.HeaderOffset + layoutcodex.ColumnHeadSize*len(columnsOrder))
	offset += d.keyEncoder.prefix.Size(offset)
	offset += d.keyEncoder.suffix.Size(offset)
	offset += d.columnEncoder.trailer.Size(offset)
	offset += d.columnEncoder.values.Size(offset)
	offset += d.columnEncoder.prefixChangedAt.Size(offset)
	offset += 1 // 1 un-used padding byte

	return offset
}

// Finish the writing to the current page, and prepare data for flushing to the storage
//
// Caller of the function must keep track of the current accumlated size of the block
// or using  DataBlockWriter.Size() function to get the size before finishing
func (d *DataBlockWriter) Finish(rows uint32, size int) (finished []byte) {
	if rows < d.rows-1 || rows > d.rows {
		panic("DataBlockWriter only accepts to finish either all rows, or [all rows minus 1]")
	}

	h := layoutcodex.NewHeader(common.TableV2, uint16(len(columnsOrder)), rows)
	d.layoutEncoder.Init(size, h)
	for _, columnName := range columnsOrder {
		switch columnName {
		case "prefix":
			d.layoutEncoder.Encode(rows, &d.keyEncoder.prefix)
		case "suffix":
			d.layoutEncoder.Encode(rows, &d.keyEncoder.suffix)
		case "trailer":
			d.layoutEncoder.Encode(rows, &d.columnEncoder.trailer)
		case "values":
			d.layoutEncoder.Encode(rows, &d.columnEncoder.values)
		case "prefixChangedAt":
			d.layoutEncoder.Encode(rows, &d.columnEncoder.prefixChangedAt)
		default:
			panic(fmt.Sprintf("Unhandled column: %s", columnName))
		}
	}

	finished = d.layoutEncoder.Data()

	return finished
}

func NewDataBlockWriter(comparer nogodb_common.IComparer) *DataBlockWriter {
	d := &DataBlockWriter{
		comparer: comparer,
	}

	d.keyEncoder = struct {
		prefix prefixbytescodex.PrefixBytesEncoder
		suffix rawbytescodex.RawByteEncoder
	}{
		prefix: prefixbytescodex.PrefixBytesEncoder{
			// TODO(low): Make this configurable
			BundleSize: defaultBundleSize,
		},
		suffix: rawbytescodex.RawByteEncoder{},
	}
	d.keyEncoder.prefix.Init()
	d.keyEncoder.suffix.Init()

	d.columnEncoder = struct {
		trailer         uintcodex.UintEncoder[uint64]
		values          rawbytescodex.RawByteEncoder
		prefixChangedAt bitmapcodex.BitmapEncoder
	}{
		trailer:         uintcodex.UintEncoder[uint64]{},
		values:          rawbytescodex.RawByteEncoder{},
		prefixChangedAt: bitmapcodex.BitmapEncoder{},
	}
	d.columnEncoder.trailer.Init()
	d.columnEncoder.values.Init()
	d.columnEncoder.prefixChangedAt.Init()

	d.layoutEncoder = layoutcodex.LayoutEncoder{}
	// we will init the layoutEncoder with the estimated size when the finish the block
	d.layoutEncoder.Reset()

	d.Reset()
	return d
}
