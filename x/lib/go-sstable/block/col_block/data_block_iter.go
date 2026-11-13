package col_block

import (
	"encoding/binary"
	"fmt"
	"slices"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	bitmapcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/bitmap_codex"
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	prefixbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/prefix_bytes_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
	"go.uber.org/zap"
)

// Note: All received keys are the full user key, eg internalKey.UserKey

type DataBlockIter struct {
	bpool    *predictable_size.PredictablePool
	comparer nogodb_common.IComparer

	keyDecoder struct {
		prefix  *prefixbytescodex.PrefixBytesDecoder
		suffix  *rawbytescodex.RawBytesDecoder
		trailer *uintcodex.UintDecoder[uint64]
	}

	prefixChangedAt *bitmapcodex.BitmapDecoder
	values          *rawbytescodex.RawBytesDecoder

	currRow uint32 // used for iterating over the block
	closed  bool
}

func (i *DataBlockIter) SeekGTE(key []byte) *nogodb_common.InternalKV {
	foundRow, _ := i.seekGTEInternal(key)
	if foundRow >= i.keyDecoder.prefix.Rows() {
		return nil
	}
	// move the cursor to the found index
	i.currRow = foundRow
	return i.toKv()
}

// SeekPrefixGTE moves the iterator to the first key/value pair whose key >= to the given key.
// that has the defined prefix for faster looking up
func (i *DataBlockIter) SeekPrefixGTE(prefix, key []byte) *nogodb_common.InternalKV {
	panic("Block Iterator doesn't support SeekPrefixGE, this kind of function should be handled in the higher level iteration")
}

// SeekLTE moves the iterator to the last key/value pair whose key ≤ to the given key.
func (i *DataBlockIter) SeekLTE(key []byte) *nogodb_common.InternalKV {
	foundRow, eq := i.seekGTEInternal(key)
	if !eq {
		foundRow -= 1
	}

	i.currRow = foundRow
	return i.toKv()
}

// First moves the iterator the first key/value pair.
func (i *DataBlockIter) First() *nogodb_common.InternalKV {
	i.currRow = 0
	return i.toKv()
}

// Last moves the iterator the last key/value pair.
func (i *DataBlockIter) Last() *nogodb_common.InternalKV {
	i.currRow = i.keyDecoder.prefix.Rows() - 1
	return i.toKv()
}

// Next moves the iterator to the next key/value pair
func (i *DataBlockIter) Next() *nogodb_common.InternalKV {
	if i.currRow == i.keyDecoder.prefix.Rows()-1 {
		return nil
	}

	i.currRow = i.currRow + 1
	return i.toKv()
}

// Prev moves the iterator to the previous key/value pair.
func (i *DataBlockIter) Prev() *nogodb_common.InternalKV {
	if i.currRow == 0 {
		return nil
	}

	i.currRow = i.currRow - 1
	return i.toKv()
}

// Close closes the iterator and returns any accumulated error. Exhausting
// all the key/value pairs in a table is not considered to be an error.
func (i *DataBlockIter) Close() error {
	i.closed = true
	i.currRow = 0
	i.keyDecoder.prefix = nil
	i.keyDecoder.suffix = nil
	i.keyDecoder.trailer = nil
	i.prefixChangedAt = nil
	i.values = nil
	return nil
}

func (i *DataBlockIter) IsClosed() bool {
	return i.closed
}

func (i *DataBlockIter) seekGTEInternal(key []byte) (foundRow uint32, eq bool) {
	prefixLen := i.comparer.Split(key)
	foundRow, eq = i.keyDecoder.prefix.SeekGTE(
		key[:prefixLen], 0, int32(i.keyDecoder.prefix.Rows()-1),
	)
	if eq {
		// seeking based on suffix. We can ensure that prefixChangedAt
		// holds only keys that are sorted in an increasing order
		nextPrefixChangedAt, _ := i.prefixChangedAt.SeekGTE(
			foundRow+1, 0, int32(i.prefixChangedAt.Rows()-1),
		)

		// because the keys come in an increasing order, so if their prefix are the same
		// from [foundRow, nextPrefixChangedAt-1], thus we can ensure the suffixes
		// are in an increasing order
		foundRow, eq = i.keyDecoder.suffix.SeekGTE(
			key[prefixLen:], int32(foundRow), int32(nextPrefixChangedAt-1),
		)
	}

	return foundRow, eq
}

// toKv converts the current row to the InternalKV
func (i *DataBlockIter) toKv() *nogodb_common.InternalKV {
	iKv := &nogodb_common.InternalKV{}
	var trailer [8]byte
	binary.LittleEndian.PutUint64(trailer[:], i.keyDecoder.trailer.Get(i.currRow))
	key := slices.Concat(
		i.keyDecoder.prefix.Get(i.currRow),
		i.keyDecoder.suffix.Get(i.currRow),
		trailer[:],
	)

	iKv.K = *nogodb_common.DeserializeKey(key)

	v := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
	v.ReserveBuffer(i.bpool, len(i.values.Get(i.currRow)))
	if err := v.SetBufferValue(i.values.Get(i.currRow)); err != nil {
		zap.L().Error("failed to set value", zap.Error(err))
	}
	iKv.V = v
	return iKv
}

func NewDataBlockIter(
	bp *predictable_size.PredictablePool,
	cp nogodb_common.IComparer,
	data *nogodb_common.InternalLazyValue,
) *DataBlockIter {
	d := &DataBlockIter{
		bpool:    bp,
		comparer: cp,
		keyDecoder: struct {
			prefix  *prefixbytescodex.PrefixBytesDecoder
			suffix  *rawbytescodex.RawBytesDecoder
			trailer *uintcodex.UintDecoder[uint64]
		}{},
	}
	// Refer to the README and data_block_writer to understand
	// the layout of the data block
	decoder := layoutcodex.NewLayoutDecoder(data.Value())
	var ok bool

	for i, columnName := range columnsOrder {
		switch columnName {
		case "prefix":
			d.keyDecoder.prefix, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				prefixbytescodex.NewPrefixBytesDecoder,
			).(*prefixbytescodex.PrefixBytesDecoder)
			if !ok {
				panic("NewDataBlockIter, failed to assert to PrefixBytesDecoder")
			}
		case "suffix":
			d.keyDecoder.suffix, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				rawbytescodex.NewRawBytesDecoder,
			).(*rawbytescodex.RawBytesDecoder)
			if !ok {
				panic("NewDataBlockIter, failed to assert to RawBytesDecoder")
			}
		case "trailer":
			d.keyDecoder.trailer, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				uintcodex.NewUintDecoder[uint64],
			).(*uintcodex.UintDecoder[uint64])
			if !ok {
				panic("NewDataBlockIter, failed to assert to UintDecoder")
			}
		case "values":
			d.values, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				rawbytescodex.NewRawBytesDecoder,
			).(*rawbytescodex.RawBytesDecoder)
			if !ok {
				panic("NewDataBlockIter, failed to assert to RawBytesDecoder")
			}
		case "prefixChangedAt":
			d.prefixChangedAt, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				bitmapcodex.NewBitmapDecoder,
			).(*bitmapcodex.BitmapDecoder)
			if !ok {
				panic("NewDataBlockIter, failed to assert to UintDecoder")
			}
		default:
			panic(fmt.Sprintf("Unhandled column: %s", columnName))
		}
	}

	return d
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*DataBlockIter)(nil)
