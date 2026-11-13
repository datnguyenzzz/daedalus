package row_block

import (
	"fmt"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/filter"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/queue"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/storage"
	"go.uber.org/zap"
)

type compressorPerBlock map[nogodb_common.BlockKind]compression.ICompression

// RowBlockWriter is an implementation of common.InternalWriter, which writes SSTables with row-oriented blocks
type RowBlockWriter struct {
	opts            options.BlockWriteOpt
	storageWriter   storage.ILayoutWriter
	dataBlock       *rowBlockBuf
	metaIndexBlock  *rowBlockBuf
	indexWriter     *indexWriter
	flushDecider    common.IFlushDecider
	comparer        nogodb_common.IComparer
	filterWriter    filter.IWriter
	compressors     compressorPerBlock
	checksumer      nogodb_common.IChecksum
	taskQueue       queue.IQueue
	tableVersion    common.TableVersion
	bytesBufferPool *predictable_size.PredictablePool
}

func (rw *RowBlockWriter) Add(key nogodb_common.InternalKey, value []byte) error {
	if err := rw.validateKey(key); err != nil {
		return err
	}

	if err := rw.mightFlush(key, len(value)); err != nil {
		return err
	}

	if rw.filterWriter != nil {
		rw.filterWriter.Add(key.UserKey)
	}

	if err := rw.dataBlock.WriteEntry(key, value); err != nil {
		return err
	}

	return nil
}

func (rw *RowBlockWriter) Close() error {
	// Flush the last (current) data block to the storage
	if rw.dataBlock.EntryCount() > 0 {
		// create an index with key ≥ currentKey
		if err := rw.doFlush(nogodb_common.InternalKey{}); err != nil {
			return err
		}
	}

	// Wait to finish all current pending data_block_flush task
	// Then close the queue
	if err := rw.taskQueue.Close(); err != nil {
		return err
	}

	// Build and Flush filter block
	if rw.filterWriter != nil {
		var rawData []byte
		rw.filterWriter.Build(&rawData)
		// compress and checksum
		compressor := rw.compressors[nogodb_common.BlockKindFilter]
		pb := block.CompressToPb(compressor, rw.checksumer, rawData)
		// write to the stable storage
		bh, err := rw.storageWriter.WritePhysicalBlock(*pb)
		if err != nil {
			zap.L().Error("failed to write filter to the storage", zap.Error(err))
			return err
		}
		// save the filter block location to the meta index block
		encodedBH := make([]byte, common.MaxBlockHandleBytes)
		n := bh.EncodeInto(encodedBH)
		err = rw.metaIndexBlock.WriteEntry(
			nogodb_common.MakeMetaIndexKey(nogodb_common.BlockKindFilter),
			encodedBH[:n],
		)
		if err != nil {
			zap.L().Error("failed to write filter to the metaIndexBlock", zap.Error(err))
			return err
		}
	}
	// Build and Flush index block to the stable storage
	{
		indexBh, err := rw.indexWriter.BuildIndex()
		if err != nil {
			zap.L().Error("failed to build/finish the index", zap.Error(err))
			return err
		}
		// save the block location of the 2-level index to the index
		// key - 1 byte indicate block kind , value - varint encoded of the block handle
		encodedBH := make([]byte, common.MaxBlockHandleBytes)
		n := indexBh.EncodeInto(encodedBH)
		err = rw.metaIndexBlock.WriteEntry(
			nogodb_common.MakeMetaIndexKey(nogodb_common.BlockKindIndex),
			encodedBH[:n],
		)
		if err != nil {
			zap.L().Error("failed to write the 2-level index block to the meta index", zap.Error(err))
		}
	}

	// write the meta index block
	metaIndexRaw := rw.bytesBufferPool.Get(rw.metaIndexBlock.EstimateSize())
	metaIndexRaw = metaIndexRaw[:rw.metaIndexBlock.EstimateSize()]
	rw.metaIndexBlock.Finish(metaIndexRaw)
	compressor := rw.compressors[nogodb_common.BlockKindIndex]
	pb := block.CompressToPb(compressor, rw.checksumer, metaIndexRaw)
	bh, err := rw.storageWriter.WritePhysicalBlock(*pb)
	if err != nil {
		zap.L().Error("failed to write meta index to the storage", zap.Error(err))
		return err
	}
	rw.bytesBufferPool.Put(metaIndexRaw)

	// write Footer
	footer := &block.Footer{
		Version:     rw.tableVersion,
		MetaIndexBH: bh,
	}
	footerRaw := footer.Serialise()
	if _, err := rw.storageWriter.WriteRawBytes(footerRaw); err != nil {
		zap.L().Error("failed to write Footer to the storage", zap.Error(err))
		return err
	}

	// Reset all buffers and close the writable file
	if err := rw.storageWriter.Finish(); err != nil {
		zap.L().Error("failed to finish the storage writer", zap.Error(err))
		return err
	}

	rw.dataBlock.CleanUpForReuse()
	rw.dataBlock.Release()
	rw.dataBlock = nil
	rw.metaIndexBlock.CleanUpForReuse()
	rw.metaIndexBlock.Release()
	rw.metaIndexBlock = nil
	rw.indexWriter.Release()
	rw.indexWriter = nil

	return nil
}

// validateKey ensure the key is added in the asc order.
func (rw *RowBlockWriter) validateKey(key nogodb_common.InternalKey) error {
	if rw.dataBlock.EntryCount() == 0 {
		return nil
	}
	lastKey := *rw.dataBlock.CurKey()
	cmp := rw.comparer.Compare(key.UserKey, lastKey.UserKey)
	if cmp < 0 || (cmp == 0 && lastKey.Trailer <= key.Trailer) {
		return fmt.Errorf("%w: keys must be added in strictly increasing order", common.ClientInvalidRequestError)
	}

	return nil
}

// mightFlush validate if required or not, if yes then flush (and compression) the data to the stable storage
func (rw *RowBlockWriter) mightFlush(key nogodb_common.InternalKey, dataLen int) error {
	// Skip if the data block is not ready to flush
	if !rw.dataBlock.ShouldFlush(key.Size(), dataLen, rw.flushDecider) {
		return nil
	}

	return rw.doFlush(key)
}

func (rw *RowBlockWriter) doFlush(key nogodb_common.InternalKey) error {
	prevKey := rw.dataBlock.CurKey()
	// 1. Finish the data block, write the serialised data into the buffer
	uncompressed := rw.bytesBufferPool.Get(rw.dataBlock.EstimateSize())
	uncompressed = uncompressed[:rw.dataBlock.EstimateSize()]
	rw.dataBlock.Finish(uncompressed)

	// 2. Get the task from the pool and compute the physical format
	// of the data block to prepare the needed input for the task
	task := block.SpawnNewTask()
	task.StorageWriter = rw.storageWriter
	task.Physical = block.CompressToPb(rw.compressors[nogodb_common.BlockKindData], rw.checksumer, uncompressed)
	// inputs for index writer
	task.IndexKey = prevKey
	task.IndexWriter = rw.indexWriter

	// 3. Put the task into queue that is running on another go-routine
	// for execution
	rw.taskQueue.Put(task)

	// 4. Put the uncompressed buffer back to the bytes buffer pool
	rw.bytesBufferPool.Put(uncompressed)
	// 5. Reset the current data block for the next subsequent writes
	rw.dataBlock.CleanUpForReuse()
	rw.dataBlock.Release()
	rw.dataBlock = newBlock(rw.opts.BlockRestartInterval, rw.bytesBufferPool, rw.opts.BlockSize)

	return nil
}

func NewRowBlockWriter(w go_fs.Writable, opts options.BlockWriteOpt, version common.TableVersion) *RowBlockWriter {
	c := compressorPerBlock{}
	for blockKind := range nogodb_common.BlockKindStrings {
		if _, ok := opts.Compression[blockKind]; !ok {
			c[blockKind] = compression.NewCompressor(opts.DefaultCompression)
			continue
		}

		c[blockKind] = compression.NewCompressor(opts.Compression[blockKind])
	}
	crc32Checksum := nogodb_common.NewChecksumer(nogodb_common.CRC32Checksum)
	comparer := nogodb_common.NewComparer()
	flushDecider := common.NewFlushDecider(opts.BlockSize, opts.BlockSizeThreshold)
	storageWriter := storage.NewLayoutWriter(w)
	bp := predictable_size.NewPredictablePool()
	metaIndexBlock := newBlock(1, bp, opts.BlockSize)
	return &RowBlockWriter{
		opts:           opts,
		storageWriter:  storageWriter,
		dataBlock:      newBlock(opts.BlockRestartInterval, bp, opts.BlockSize),
		metaIndexBlock: metaIndexBlock,
		indexWriter: newIndexWriter(
			comparer,
			c[nogodb_common.BlockKindIndex],
			crc32Checksum,
			flushDecider,
			storageWriter,
			metaIndexBlock,
			bp,
			opts,
		),
		comparer:        comparer,
		filterWriter:    filter.NewFilterWriter(filter.BloomFilter), // Use bloom filter as a default method
		flushDecider:    flushDecider,
		compressors:     c,
		checksumer:      crc32Checksum,
		taskQueue:       queue.NewQueue(0, false),
		tableVersion:    version,
		bytesBufferPool: bp,
	}
}

var _ nogodb_common.InternalWriter = (*RowBlockWriter)(nil)
