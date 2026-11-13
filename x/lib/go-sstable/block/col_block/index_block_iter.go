package col_block

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
	uintcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/uint_codex"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"go.uber.org/zap"
)

type IndexBlockIter struct {
	bpool    *predictable_size.PredictablePool
	comparer nogodb_common.IComparer

	// index block key's only contain UserKeys
	keyDecoder *rawbytescodex.RawBytesDecoder

	blockHandleDecoder struct {
		offset *uintcodex.UintDecoder[uint64]
		length *uintcodex.UintDecoder[uint64]
	}

	currRow uint32 // used for iterating over the block
	closed  bool
}

func (i *IndexBlockIter) SeekGTE(key []byte) *nogodb_common.InternalKV {
	foundRow, _ := i.seekGTEInternal(key)
	if foundRow >= i.keyDecoder.Rows() {
		return nil
	}
	// move the cursor to the found index
	i.currRow = foundRow
	return i.toKv()
}

// SeekPrefixGTE moves the iterator to the first key/value pair whose key >= to the given key.
// that has the defined prefix for faster looking up
func (i *IndexBlockIter) SeekPrefixGTE(prefix, key []byte) *nogodb_common.InternalKV {
	panic("Block Iterator doesn't support SeekPrefixGE, this kind of function should be handled in the higher level iteration")
}

// SeekLTE moves the iterator to the last key/value pair whose key ≤ to the given key.
func (i *IndexBlockIter) SeekLTE(key []byte) *nogodb_common.InternalKV {
	foundRow, eq := i.seekGTEInternal(key)
	if !eq {
		foundRow -= 1
	}

	i.currRow = foundRow
	return i.toKv()
}

// First moves the iterator the first key/value pair.
func (i *IndexBlockIter) First() *nogodb_common.InternalKV {
	i.currRow = 0
	return i.toKv()
}

// Last moves the iterator the last key/value pair.
func (i *IndexBlockIter) Last() *nogodb_common.InternalKV {
	i.currRow = i.keyDecoder.Rows() - 1
	return i.toKv()
}

// Next moves the iterator to the next key/value pair
func (i *IndexBlockIter) Next() *nogodb_common.InternalKV {
	if i.currRow == i.keyDecoder.Rows()-1 {
		return nil
	}

	i.currRow = i.currRow + 1
	return i.toKv()
}

// Prev moves the iterator to the previous key/value pair.
func (i *IndexBlockIter) Prev() *nogodb_common.InternalKV {
	if i.currRow == 0 {
		return nil
	}

	i.currRow = i.currRow - 1
	return i.toKv()
}

// Close closes the iterator and returns any accumulated error. Exhausting
// all the key/value pairs in a table is not considered to be an error.
func (i *IndexBlockIter) Close() error {
	i.closed = true
	i.currRow = 0
	i.keyDecoder = nil
	i.blockHandleDecoder.offset = nil
	i.blockHandleDecoder.length = nil
	return nil
}

func (i *IndexBlockIter) IsClosed() bool {
	return i.closed
}

func (i *IndexBlockIter) seekGTEInternal(key []byte) (foundRow uint32, eq bool) {
	// we always ensure that all keys of the index blocks
	// are sorted in an increasing order
	return i.keyDecoder.SeekGTE(key, 0, int32(i.keyDecoder.Rows())-1)
}

// toKv converts the current row to the InternalKV
func (i *IndexBlockIter) toKv() *nogodb_common.InternalKV {
	iKv := &nogodb_common.InternalKV{
		K: nogodb_common.InternalKey{},
	}
	iKv.K.UserKey = i.keyDecoder.Get(i.currRow)

	bh := &common.BlockHandle{
		Offset: i.blockHandleDecoder.offset.Get(i.currRow),
		Length: i.blockHandleDecoder.length.Get(i.currRow),
	}

	var buf [common.MaxBlockHandleBytes]byte
	_ = bh.EncodeInto(buf[:])

	v := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
	v.ReserveBuffer(i.bpool, len(buf))
	if err := v.SetBufferValue(buf[:]); err != nil {
		zap.L().Error("failed to set value", zap.Error(err))
	}
	iKv.V = v
	return iKv
}

func NewIndexBlockIter(
	bp *predictable_size.PredictablePool,
	cp nogodb_common.IComparer,
	data *nogodb_common.InternalLazyValue,
) *IndexBlockIter {
	d := &IndexBlockIter{
		bpool:    bp,
		comparer: cp,
		blockHandleDecoder: struct {
			offset *uintcodex.UintDecoder[uint64]
			length *uintcodex.UintDecoder[uint64]
		}{},
	}

	// Refer to the README and data_block_writer to understand
	// the layout of the data block
	decoder := layoutcodex.NewLayoutDecoder(data.Value())
	var ok bool

	for i, cName := range indexColumnsOrder {
		switch cName {
		case "key":
			d.keyDecoder, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				rawbytescodex.NewRawBytesDecoder,
			).(*rawbytescodex.RawBytesDecoder)
			if !ok {
				panic("NewIndexBlockIter, failed to assert to RawBytesDecoder")
			}
		case "offset":
			d.blockHandleDecoder.offset, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				uintcodex.NewUintDecoder[uint64],
			).(*uintcodex.UintDecoder[uint64])
			if !ok {
				panic("NewIndexBlockIter, failed to assert to UintDecoder")
			}
		case "length":
			d.blockHandleDecoder.length, ok = layoutcodex.Decode(
				cp,
				decoder,
				uint16(i),
				uintcodex.NewUintDecoder[uint64],
			).(*uintcodex.UintDecoder[uint64])
			if !ok {
				panic("NewIndexBlockIter, failed to assert to UintDecoder")
			}
		default:
			panic("IndexBlockWriter unhandled column name")
		}
	}

	return d
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*IndexBlockIter)(nil)
