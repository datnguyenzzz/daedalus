package db

// Inspiration by PebbleDB and RocksDB

// In RocksDB, a batch is the unit for all write operations. Even writing
// a single key is transformed internally to a batch. The batch internal
// representation is a contiguous byte buffer with a fixed 12-byte
// header, followed by a series of records.

// ```
//   +------------+-----------+--- ... ---+
//   | SeqNum (8) | Count (4) |  Records  |
//   +------------+-----------+--- ... ---+
// ```

// Each record has a 1-byte kind tag prefix, followed by 1 or 2 length
// prefixed strings (varstring):

// ```
//   +----------+-----------------+-------------------+
//   | Kind (1) | Key (varstring) | Value (varstring) |
//   +----------+-----------------+-------------------+
// ```

// For example a real batches would look like
// ```
// +-----------------------------------------------+
// | Batch(SeqNum=0,Count=9,Records= <9 records>)  |
// +-----------------------------------------------+
// | Batch(SeqNum=9,Count=5,Records= <5 records>)  |
// +-----------------------------------------------+
// | Batch(SeqNum=14,Count=7,Records <7 records>)  |
// +-----------------------------------------------+
// | ...                              		       |
// +-----------------------------------------------+
// ```

// (The `Kind` indicates if there are 1 or 2 varstrings. `Set`, `Merge`,
// and `DeleteRange` have 2 varstrings, while `Delete` has 1.)

// Adding a mutation to a batch involves appending a new record to the
// buffer. This format is extremely fast for writes, but the lack of
// indexing makes it untenable to use directly for reads. In order to
// support iteration and reading, a separate indexing structure is needed.

// Both RocksDB and PebbleDB use a skiplist for the indexing structure.
// But rather than the skiplist storing a copy of the key, it simply stores
// the offset of the record within the mutation buffer. The iteration order
// for this map is constructed so that records sort on key, and for equal
// keys they sort on descending offset. Newer records for the same key appear
// before older records.

import (
	"context"
	"encoding/binary"
	"io"
	"sync"
	"sync/atomic"
	"unsafe"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_art "github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree"
)

const (
	maxRetainSize  = 1 << 20 // 1 MB
	BatchHeaderLen = 12
	countOffset    = 8
)

type Batch struct {
	batchInternal
	applied atomic.Bool
}

type batchInternal struct {
	buf []byte
	cmp nogodb_common.IComparer
	db  *DB
	// The count of records in the batch. This count will be stored in the batch
	// data whenever Repr() is called.
	count uint32
	index nogodb_art.ITree[any]

	// TODO(high): In PebbleDB, when the batch is too large that can not fit into
	// a memtable, it would be marked as <immutable>, return as a "flushable" and
	// will be put into the <db.mem.flushableQueue>

	commitErr error

	// committing is set to true when a batch begins to commit. It's used to
	// ensure the batch is not mutated concurrently
	committing atomic.Bool
}

var (
	_ IReader = (*Batch)(nil)
	_ IWriter = (*Batch)(nil)
)

var batchPool sync.Pool = sync.Pool{
	New: func() any {
		return &Batch{}
	},
}

func newBatch(db *DB, needIndexing bool) *Batch {
	b := batchPool.Get().(*Batch)
	b.db = db
	b.index = nil
	if needIndexing {
		b.index = nogodb_art.NewTree[any](context.TODO())
	}
	return b
}

// init create an initial b.buf, starting with BatchHeaderLen zeroed bytes
func (b *Batch) init(size int) {
	n := 1
	for n < size {
		n *= 2
	}
	if cap(b.buf) < n {
		b.buf = make([]byte, BatchHeaderLen, n)
	}

	b.buf = b.buf[:BatchHeaderLen]
	clear(b.buf)
}

// grow extends the b.buf to <n> more bytes
func (b *Batch) grow(n int) {
	newSize := len(b.buf) + n
	if newSize > cap(b.buf) {
		newCap := 2 * cap(b.buf)
		for newCap < newSize {
			newCap *= 2
		}
		newData := make([]byte, len(b.buf), newCap)
		copy(newData, b.buf)
		b.buf = newData
	}
	b.buf = b.buf[:newSize]
}

// WRITER \\

func (b *Batch) Delete(key []byte) error {
	_ = b.put(key, nil, nogodb_common.KeyKindDelete)
	if b.index != nil {
		if _, err := b.index.Insert(context.TODO(), b.cmp.AbbreviatedKey(key), nil); err != nil {
			return err
		}
	}

	return nil
}

func (b *Batch) Set(key, value []byte) error {
	_ = b.put(key, value, nogodb_common.KeyKindSet)
	if b.index != nil {
		if _, err := b.index.Insert(context.TODO(), b.cmp.AbbreviatedKey(key), nil); err != nil {
			return err
		}
	}

	return nil
}

// Utilities \\

type Header struct {
	// SeqNum is the sequence number at which the batch is committed. A batch
	// that has not yet committed will have a zero sequence number.
	SeqNum nogodb_common.SeqNum
	// Count is the count of keys written to the batch.
	Count uint32
}

func readHeader(repr []byte) (h Header, ok bool) {
	if len(repr) < BatchHeaderLen {
		return h, false
	}
	return Header{
		SeqNum: nogodb_common.SeqNum(binary.LittleEndian.Uint64(repr[:countOffset])),
		Count:  binary.LittleEndian.Uint32(repr[countOffset:BatchHeaderLen]),
	}, true
}

func (b *Batch) SeqNum() nogodb_common.SeqNum {
	header, ok := readHeader(b.buf)
	if !ok {
		return 0
	}

	return header.SeqNum
}

// SetCountAndReturn fills the record counts into the header area
// and return the buffer
func (b *Batch) SetCountToHeader() {
	binary.LittleEndian.PutUint32(b.buf[countOffset:BatchHeaderLen], b.count)
}

func (b *Batch) SetSeqNumToHeader(seqNum uint64) {
	binary.LittleEndian.PutUint64(b.buf[:countOffset], seqNum)
}

func (b *Batch) Count() uint32 {
	return b.count
}

// READER \\

func (b *Batch) Close() error {
	b.reset()
	batchPool.Put((*Batch)(unsafe.Pointer(b)))
	return nil
}

func (b *Batch) Get(key []byte) (value []byte, closer io.Closer, err error) {
	panic("unimplemented")
}

// Internal \\

func (b *Batch) reset() {
	b.batchInternal = batchInternal{
		buf:   b.buf,
		index: b.index,
	}
	b.applied.Store(false)
	if b.index != nil {
		b.index = nogodb_art.NewTree[any](context.Background())
	}
	if cap(b.buf) > maxRetainSize {
		b.buf = nil
	} else {
		b.buf = b.buf[:0]
	}
}

// put key,value and keyKind into the buf without indexing, return the offset within b.buf
// before the (key, value, keyKind) was put
func (b *Batch) put(key, value []byte, kind nogodb_common.KeyKind) (offset int) {
	if b.committing.Load() {
		panic("batch.Put failed, batch is already commiting")
	}

	if len(b.buf) == 0 {
		b.init(BatchHeaderLen)
	}

	b.count += 1
	offset = len(b.buf)
	prev := offset
	if len(value) > 0 {
		b.grow(len(key) + len(value) + 2*binary.MaxVarintLen32 + 1)
	} else {
		b.grow(len(key) + binary.MaxVarintLen32 + 1)
	}
	b.buf[offset] = byte(kind)
	offset += 1
	// encoding key to the b.buf
	offset = binary.PutUvarint(b.buf, uint64(len(key)))
	copy(b.buf[offset:offset+len(key)], key)
	offset += len(key)

	if len(value) > 0 {
		// encoding value to the b.buf
		offset = binary.PutUvarint(b.buf, uint64(len(value)))
		copy(b.buf[offset:offset+len(value)], value)
		offset += len(value)
	}

	b.buf = b.buf[:offset]

	return prev
}
