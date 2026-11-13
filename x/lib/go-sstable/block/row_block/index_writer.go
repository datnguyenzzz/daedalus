package row_block

import (
	"encoding/binary"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/storage"
)

const (
	estimateNumberOfTopLevelBlocks = 128
)

type firstLevelIndex struct {
	key           *nogodb_common.InternalKey
	entries       int
	finishedBlock []byte
}

// indexWriter The `i'th` value is the encoded block handle of the `i'th` data block.
// The `i'th` key is a string `>=` last key in that data block and `<` the first key
// in the successive data block.
//
// It consists of a sequence of lower-level
// index blocks with block handles for data blocks followed by a single top-level
// index block with block handles for the lower-level index blocks.
type indexWriter struct {
	firstLevelBlock   *rowBlockBuf
	secondLevelBlock  *rowBlockBuf
	comparer          nogodb_common.IComparer
	compressor        compression.ICompression
	checksumer        nogodb_common.IChecksum
	flushDecider      common.IFlushDecider
	storageWriter     storage.ILayoutWriter
	firstLevelIndices []*firstLevelIndex
	metaIndexBlock    *rowBlockBuf
	bytesBufferPool   *predictable_size.PredictablePool
	opts              *options.BlockWriteOpt
}

func (w *indexWriter) Add(key *nogodb_common.InternalKey, bh *common.BlockHandle) error {
	if bh.Length == 0 {
		return nil
	}
	if err := w.mightFlushToMem(key); err != nil {
		return err
	}
	encoded := make([]byte, common.MaxBlockHandleBytes)
	n := bh.EncodeInto(encoded)
	encoded = encoded[:n]
	return w.firstLevelBlock.WriteEntry(*key, encoded)
}

func (w *indexWriter) mightFlushToMem(key *nogodb_common.InternalKey) error {
	estimatedBHSize := binary.MaxVarintLen64 * 2
	if !w.firstLevelBlock.ShouldFlush(key.Size(), estimatedBHSize, w.flushDecider) {
		return nil
	}

	// Start flushing the index block to the top level of the index block
	// As this function will only be triggered as a part of the DataBlock.Flush()
	// Therefore we can always assure that they are no other concurrent attempts

	// Instead of flushing directly to the storage, we store the top level indices
	// into the memory, we will only write the index blocks once the table is finished.
	// We do it because based on the design, the index blocks are only written after all
	// of data blocks are flushed, and the table is about closing.
	// Therefore the layout would look like: (refero to README for more:)
	//   [data blocks]
	//   [1st level index blocks]
	//   [2nd level index block]
	// TODO (low): Open questions:
	//   1. How to recover the indices if the machine crash ?
	w.flushFirstLevelIndexToMem(key)
	return nil
}

func (w *indexWriter) flushFirstLevelIndexToMem(key *nogodb_common.InternalKey) {
	prevKey := w.firstLevelBlock.CurKey()
	idx := &firstLevelIndex{
		key:     prevKey,
		entries: w.firstLevelBlock.EntryCount(),
	}
	uncompressed := w.bytesBufferPool.Get(w.firstLevelBlock.EstimateSize())
	uncompressed = uncompressed[:w.firstLevelBlock.EstimateSize()]
	w.firstLevelBlock.Finish(uncompressed)
	idx.finishedBlock = make([]byte, len(uncompressed))
	copy(idx.finishedBlock, uncompressed)
	w.firstLevelIndices = append(w.firstLevelIndices, idx)
	w.bytesBufferPool.Put(uncompressed)
	// reset the current first level index block for the next subsequent writes
	w.firstLevelBlock.CleanUpForReuse()
	w.firstLevelBlock.Release()
	w.firstLevelBlock = newBlock(1, w.bytesBufferPool, w.opts.BlockSize)
}

// buildIndex build the 2-level index for the SST
func (w *indexWriter) BuildIndex() (*common.BlockHandle, error) {
	// flush all of pending/un-finished 1-level indices to mem
	w.flushFirstLevelIndexToMem(w.firstLevelBlock.CurKey())
	for _, idx := range w.firstLevelIndices {
		// 1. Write the compressed first level index to the storage
		pb := block.CompressToPb(w.compressor, w.checksumer, idx.finishedBlock)
		bh, err := w.storageWriter.WritePhysicalBlock(*pb)
		if err != nil {
			return nil, err
		}
		// 2. Write the encoded value of the 1-level index block handle
		// into buffer
		encodedBH := make([]byte, common.MaxBlockHandleBytes)
		n := bh.EncodeInto(encodedBH)
		if err := w.secondLevelBlock.WriteEntry(*idx.key, encodedBH[:n]); err != nil {
			return nil, err
		}
	}

	// 3. Build and Write the 2-level index buffer to the storage
	uncompressed := w.bytesBufferPool.Get(w.secondLevelBlock.EstimateSize())
	uncompressed = uncompressed[:w.secondLevelBlock.EstimateSize()]
	w.secondLevelBlock.Finish(uncompressed)
	pb := block.CompressToPb(w.compressor, w.checksumer, uncompressed)
	bh, err := w.storageWriter.WritePhysicalBlock(*pb)
	if err == nil {
		return &bh, nil
	}

	w.bytesBufferPool.Put(uncompressed)
	return nil, err
}

func (w *indexWriter) Release() {
	w.firstLevelBlock.CleanUpForReuse()
	w.firstLevelBlock.Release()
	clear(w.firstLevelIndices)
}

func newIndexWriter(
	comparer nogodb_common.IComparer,
	compressor compression.ICompression,
	checksumer nogodb_common.IChecksum,
	flushDecider common.IFlushDecider,
	storageWriter storage.ILayoutWriter,
	metaIndexBlock *rowBlockBuf,
	bufferPool *predictable_size.PredictablePool,
	opts options.BlockWriteOpt,
) *indexWriter {
	return &indexWriter{
		// The index block also use the row oriented layout.
		// And its restart interval is 1, aka every entry is a restart point.
		firstLevelBlock:   newBlock(1, bufferPool, opts.BlockSize),
		secondLevelBlock:  newBlock(1, bufferPool, opts.BlockSize),
		comparer:          comparer,
		compressor:        compressor,
		checksumer:        checksumer,
		flushDecider:      flushDecider,
		storageWriter:     storageWriter,
		firstLevelIndices: make([]*firstLevelIndex, 0, estimateNumberOfTopLevelBlocks),
		metaIndexBlock:    metaIndexBlock,
		bytesBufferPool:   bufferPool,
		opts:              &opts,
	}
}

var _ block.IIndexWriter = (*indexWriter)(nil)
