package col_block

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
	"go.uber.org/zap"
)

type KVBlockIter struct {
	bp   *predictable_size.PredictablePool
	keys *rawbytescodex.RawBytesDecoder
	// values store the encoded blochHandle
	values  *rawbytescodex.RawBytesDecoder
	currRow uint32 // used for iterating over the block
	closed  bool
}

func (i *KVBlockIter) SeekGTE(key []byte) *nogodb_common.InternalKV {
	panic("KVBlockIter can't do this")
}

func (i *KVBlockIter) SeekPrefixGTE(prefix, key []byte) *nogodb_common.InternalKV {
	panic("KVBlockIter can't do this")
}

func (i *KVBlockIter) SeekLTE(key []byte) *nogodb_common.InternalKV {
	panic("KVBlockIter can't do this")
}

// First moves the iterator the first key/value pair.
func (i *KVBlockIter) First() *nogodb_common.InternalKV {
	i.currRow = 0
	return i.toKv()
}

// Last moves the iterator the last key/value pair.
func (i *KVBlockIter) Last() *nogodb_common.InternalKV {
	i.currRow = i.keys.Rows() - 1
	return i.toKv()
}

// Next moves the iterator to the next key/value pair
func (i *KVBlockIter) Next() *nogodb_common.InternalKV {
	if i.currRow == i.keys.Rows()-1 {
		return nil
	}

	i.currRow = i.currRow + 1
	return i.toKv()
}

// Prev moves the iterator to the previous key/value pair.
func (i *KVBlockIter) Prev() *nogodb_common.InternalKV {
	if i.currRow == 0 {
		return nil
	}

	i.currRow = i.currRow - 1
	return i.toKv()
}

func (i *KVBlockIter) Close() error {
	i.closed = true
	i.currRow = 0
	i.keys = nil
	i.values = nil
	return nil
}

func (i *KVBlockIter) IsClosed() bool {
	return i.closed
}

// toKv converts the current row to the InternalKV
func (i *KVBlockIter) toKv() *nogodb_common.InternalKV {
	iKv := &nogodb_common.InternalKV{
		K: nogodb_common.InternalKey{},
	}
	// Meta block always write full common.internalKey
	iKv.K = *nogodb_common.DeserializeKey(i.keys.Get(i.currRow))
	buf := i.values.Get(i.currRow)
	v := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
	v.ReserveBuffer(i.bp, len(buf))
	if err := v.SetBufferValue(buf[:]); err != nil {
		zap.L().Error("failed to set value", zap.Error(err))
	}
	iKv.V = v
	return iKv
}

func NewKVBlockIter(
	bp *predictable_size.PredictablePool,
	cp nogodb_common.IComparer,
	data *nogodb_common.InternalLazyValue,
) *KVBlockIter {
	kv := &KVBlockIter{
		bp: bp,
	}
	decoder := layoutcodex.NewLayoutDecoder(data.Value())

	var ok bool
	kv.keys, ok = layoutcodex.Decode(
		cp,
		decoder,
		0,
		rawbytescodex.NewRawBytesDecoder,
	).(*rawbytescodex.RawBytesDecoder)
	if !ok {
		panic("NewKVBlockIter, failed to assert to RawBytesDecoder")
	}

	kv.values, ok = layoutcodex.Decode(
		cp,
		decoder,
		1,
		rawbytescodex.NewRawBytesDecoder,
	).(*rawbytescodex.RawBytesDecoder)
	if !ok {
		panic("NewKVBlockIter, failed to assert to RawBytesDecoder")
	}

	return kv
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*KVBlockIter)(nil)
