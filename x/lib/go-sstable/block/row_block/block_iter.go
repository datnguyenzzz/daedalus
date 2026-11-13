package row_block

import (
	"encoding/binary"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	"go.uber.org/zap"
)

// BlockIterator is an iterator over a single row-based block.
// BlockIterator will still return even if the record has tombstone mark
type BlockIterator struct {
	bpool *predictable_size.PredictablePool
	// data represents entire data of the block
	data *nogodb_common.InternalLazyValue
	// key represents key of the current entry
	key []byte
	// value represents value of the current entry
	value []byte
	// offsets
	offset        uint64
	nextOffset    uint64
	trailerOffset uint64
	// restarts
	numRestarts   int32
	restartPoints []int32
	// auxiliary
	cmp nogodb_common.IComparer
}

func (i *BlockIterator) SeekPrefixGTE(prefix, key []byte) *nogodb_common.InternalKV {
	panic("Block Iterator doesn't support SeekPrefixGE, this kind of function should be handled in the higher level iteration")
}

func (i *BlockIterator) SeekGTE(key []byte) *nogodb_common.InternalKV {
	lo, hi := int32(0), i.numRestarts-1
	var pos int32
	for lo <= hi {
		mid := (lo + hi) >> 1

		// decode the first key at the restart point
		blkOffset := i.restartPoints[mid]
		_, e := binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)
		unsharedLen, e := binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)
		_, e = binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)

		k := i.data.Value()[blkOffset : uint64(blkOffset)+unsharedLen]
		if i.cmp.Compare(k, key) <= 0 {
			pos = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	i.offset = uint64(i.restartPoints[pos])
	i.readEntry()
	for i.cmp.Compare(i.key, key) < 0 {
		if next := i.Next(); next == nil {
			return nil
		}
	}

	return i.toKV()
}

func (i *BlockIterator) SeekLTE(key []byte) *nogodb_common.InternalKV {
	lo, hi := int32(0), i.numRestarts-1
	pos := int32(-1)
	for lo <= hi {
		mid := (lo + hi) >> 1

		// decode the first key at the restart point
		blkOffset := i.restartPoints[mid]
		_, e := binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)
		unsharedLen, e := binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)
		_, e = binary.Uvarint(i.data.Value()[blkOffset:])
		blkOffset += int32(e)

		k := i.data.Value()[blkOffset : uint64(blkOffset)+unsharedLen]
		if i.cmp.Compare(k, key) >= 0 {
			pos = mid
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	if pos == -1 {
		_ = i.Last()
	} else {
		i.offset = uint64(i.restartPoints[pos])
	}
	i.readEntry()
	for i.cmp.Compare(i.key, key) > 0 {
		if i.atTheFirst() {
			return nil
		}

		_ = i.Prev()
	}

	return i.toKV()
}

func (i *BlockIterator) First() *nogodb_common.InternalKV {
	i.readEntry()
	return i.toKV()
}

func (i *BlockIterator) Last() *nogodb_common.InternalKV {
	// move offset to the last restart point
	i.offset = uint64(i.restartPoints[len(i.restartPoints)-1])
	i.readEntry()
	for i.nextOffset != i.trailerOffset {
		i.offset = i.nextOffset
		i.readEntry()
	}
	return i.toKV()
}

// Next move the cursor to the next entry position of the block
// returns nil if the current position is already at the end
func (i *BlockIterator) Next() *nogodb_common.InternalKV {
	if i.atTheEnd() {
		// already at the endpoint of the block
		return nil
	}
	i.offset = i.nextOffset
	i.readEntry()
	return i.toKV()
}

func (i *BlockIterator) Prev() *nogodb_common.InternalKV {
	if i.atTheFirst() {
		return nil
	}

	// max restart point that < i.offset
	lo, hi := int32(0), i.numRestarts-1
	restartPoint := -1
	for lo <= hi {
		mid := (lo + hi) >> 1
		if uint64(i.restartPoints[mid]) < i.offset {
			restartPoint = int(i.restartPoints[mid])
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	if restartPoint == -1 {
		zap.L().Warn("pointer is already at the First() offset")
		i.readEntry()
		return i.toKV()
	}

	targetOffset := i.offset
	i.offset = uint64(restartPoint)
	i.readEntry()

	for i.nextOffset != targetOffset {
		i.offset = i.nextOffset
		i.readEntry()
	}
	return i.toKV()
}

func (i *BlockIterator) Close() error {
	i.data.Release()
	i.data = nil
	i.key = nil
	i.value = nil
	i.offset = 0
	i.nextOffset = 0
	i.numRestarts = 0
	i.restartPoints = nil
	return nil
}

func (i *BlockIterator) IsClosed() bool {
	return i.data == nil
}

func (i *BlockIterator) toKV() *nogodb_common.InternalKV {
	iKV := &nogodb_common.InternalKV{}
	iKV.K = *nogodb_common.DeserializeKey(i.key)
	v := nogodb_common.NewBlankInternalLazyValue(nogodb_common.ValueFromBuffer)
	v.ReserveBuffer(i.bpool, len(i.value))
	if err := v.SetBufferValue(i.value); err != nil {
		zap.L().Error("failed to set value", zap.Error(err))
	}
	iKV.V = v
	return iKV
}

func (i *BlockIterator) atTheEnd() bool {
	return i.nextOffset == i.trailerOffset
}

func (i *BlockIterator) atTheFirst() bool {
	return i.offset == 0
}

// readEntry read key, value and nextOffset of the current entry where the iterator points at
func (i *BlockIterator) readEntry() {
	blkOffset := i.offset
	sharedLen, e := binary.Uvarint(i.data.Value()[blkOffset:])
	blkOffset += uint64(e)
	unsharedLen, e := binary.Uvarint(i.data.Value()[blkOffset:])
	blkOffset += uint64(e)
	valueLen, e := binary.Uvarint(i.data.Value()[blkOffset:])
	blkOffset += uint64(e)
	if len(i.key) == 0 {
		// we only allocate new memory at the very first of the block
		i.key = make([]byte, unsharedLen)
		copy(i.key, i.data.Value()[blkOffset:blkOffset+unsharedLen])
	} else {
		i.key = append(i.key[:sharedLen], i.data.Value()[blkOffset:blkOffset+unsharedLen]...)
	}
	blkOffset += unsharedLen
	i.value = i.data.Value()[blkOffset : blkOffset+valueLen]
	blkOffset += valueLen
	i.nextOffset = blkOffset
}

// NewBlockIterator creates an iterator over a given data block
// and set the cursor at the first position
func NewBlockIterator(
	bpool *predictable_size.PredictablePool,
	cmp nogodb_common.IComparer,
	data *nogodb_common.InternalLazyValue,
) *BlockIterator {
	// refer to the README to understand the data layout
	block := data.Value()
	numRestarts := int32(binary.LittleEndian.Uint32(block[len(block)-4:]))
	trailerOffset := uint64(len(block)) - uint64(4*numRestarts) - 4
	restartPoints := make([]int32, numRestarts)
	for i := range numRestarts {
		restartPoints[i] = int32(binary.LittleEndian.Uint32(block[trailerOffset+uint64(4*i):]))
	}

	i := &BlockIterator{
		bpool:         bpool,
		cmp:           cmp,
		data:          data,
		numRestarts:   numRestarts,
		trailerOffset: trailerOffset,
		restartPoints: restartPoints,
		offset:        0,
	}
	return i
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*BlockIterator)(nil)
