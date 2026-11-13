package col_block

import (
	layoutcodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/layout_codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
)

const (
	kvTotalColumns = 2
)

type KVBlockWriter struct {
	keys          rawbytescodex.RawByteEncoder
	values        rawbytescodex.RawByteEncoder
	layoutEncoder layoutcodex.LayoutEncoder
	rows          uint32
}

func (k *KVBlockWriter) Reset() {
	k.keys.Reset()
	k.values.Reset()
	k.layoutEncoder.Reset()
	k.rows = 0
}

func (k *KVBlockWriter) Init() {
	k.keys.Init()
	k.values.Init()
	k.layoutEncoder.Reset()
	k.rows = 0
}

func (k *KVBlockWriter) Rows() uint32 {
	return k.rows
}

func (k *KVBlockWriter) Add(key, value []byte) {
	k.rows += 1
	k.keys.Append(key)
	k.values.Append(value)
}

func (k *KVBlockWriter) Size() uint32 {
	offset := uint32(layoutcodex.HeaderOffset + layoutcodex.ColumnHeadSize*kvTotalColumns)
	offset += k.keys.Size(offset)
	offset += k.values.Size(offset)
	offset += 1

	return offset
}

func (k *KVBlockWriter) Finish(rows uint32, size int) []byte {
	if rows < k.rows-1 || rows > k.rows {
		panic("KVBlockWriter only accepts to finish either all rows, or [all rows minus 1]")
	}
	header := layoutcodex.NewHeader(1, kvTotalColumns, rows)

	k.layoutEncoder.Init(size, header)
	k.layoutEncoder.Encode(rows, &k.keys)
	k.layoutEncoder.Encode(rows, &k.values)

	return k.layoutEncoder.Data()
}

func NewKVBlockWriter() *KVBlockWriter {
	kv := &KVBlockWriter{
		keys:          rawbytescodex.RawByteEncoder{},
		values:        rawbytescodex.RawByteEncoder{},
		layoutEncoder: layoutcodex.LayoutEncoder{},
	}

	kv.Init()
	return kv
}
