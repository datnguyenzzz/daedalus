package col_block

import (
	"fmt"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/filter"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/queue"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/storage"
)

const (
	flushQueueLen        = 100_000
	maxBlockRetainedSize = 256 << 10
)

type ColBlockWriter struct {
	opt          options.BlockWriteOpt
	tableVersion common.TableVersion
	uncompressed []byte

	// storage
	storageWriter storage.ILayoutWriter

	// flush queue
	taskQueue queue.IQueue

	// filter
	filterWriter filter.IWriter

	// utilities
	flushDecider common.IFlushDecider
	comparer     nogodb_common.IComparer
	compressors  compression.ICompression
	checksumer   nogodb_common.IChecksum

	// data and indexes
	dataBlock  *DataBlockWriter
	indexBlock block.IIndexWriter

	metaIndexBlock *KVBlockWriter
}

// Add adds a key-value pair to the sstable.
func (c *ColBlockWriter) Add(key nogodb_common.InternalKey, value []byte) error {
	if err := c.validate(key); err != nil {
		return err
	}

	sizeBefore := c.dataBlock.Size()
	keyBefore := c.dataBlock.CurrKey()
	c.dataBlock.Add(key, value)
	// we will check and flush the data block without the last key
	if c.flushDecider.ShouldFlush(int(sizeBefore), int(c.dataBlock.Size())) {
		if err := c.doFlushWithoutLastKey(int(sizeBefore), keyBefore, &key); err != nil {
			return err
		}

		// after flushing, the data block will be reset, so we need re-adding the current kv
		c.dataBlock.Add(key, value)
	}

	if c.filterWriter != nil {
		// write the actual key, exclude the MVCC suffix
		prefix := c.comparer.Split(key.UserKey)
		c.filterWriter.Add(key.UserKey[:prefix])
	}

	return nil
}

// Close finishes writing the table and closes the underlying file that the table was written to.
func (c *ColBlockWriter) Close() error {
	if c.dataBlock.Rows() > 0 {
		if err := c.doFlushAll(); err != nil {
			return err
		}
	}
	var err error
	// wait for all pending flush tasks to finish, then close the queue
	if err := c.taskQueue.Close(); err != nil {
		return err
	}

	var sharedBuf []byte

	// Build and Flush filter block
	{
		if c.filterWriter != nil {
			var rawData []byte
			c.filterWriter.Build(&rawData)
			pb := block.CompressToPb(c.compressors, c.checksumer, rawData)
			bh, err := c.storageWriter.WritePhysicalBlock(*pb)
			if err != nil {
				return err
			}

			encodedBH := make([]byte, common.MaxBlockHandleBytes)
			n := bh.EncodeInto(encodedBH)
			filterMetaKey := nogodb_common.MakeMetaIndexKey(nogodb_common.BlockKindFilter)
			sz := filterMetaKey.Size()
			if cap(sharedBuf) < sz {
				block.GrowSize(&sharedBuf, sz)
			}

			filterMetaKey.SerializeTo(sharedBuf)
			c.metaIndexBlock.Add(sharedBuf, encodedBH[:n])
		}
	}
	// Build and Flush index block to the stable storage
	{
		indexBh, err := c.indexBlock.BuildIndex()
		if err != nil {
			return err
		}

		encodedBH := make([]byte, common.MaxBlockHandleBytes)
		n := indexBh.EncodeInto(encodedBH)
		indexMetaKey := nogodb_common.MakeMetaIndexKey(nogodb_common.BlockKindIndex)
		sz := indexMetaKey.Size()
		if cap(sharedBuf) < sz {
			block.GrowSize(&sharedBuf, sz)
		}
		indexMetaKey.SerializeTo(sharedBuf)
		c.metaIndexBlock.Add(sharedBuf, encodedBH[:n])
	}
	// Build and Flush meta index block to the stable storage
	var metaBh common.BlockHandle
	{
		metaSize := int(c.metaIndexBlock.Size())
		block.GrowSize(&c.uncompressed, metaSize)
		c.uncompressed = c.metaIndexBlock.Finish(c.metaIndexBlock.Rows(), metaSize)
		pb := block.CompressToPb(c.compressors, c.checksumer, c.uncompressed)
		metaBh, err = c.storageWriter.WritePhysicalBlock(*pb)
		if err != nil {
			return err
		}

		c.uncompressed = nil
	}
	// Write footer
	{
		footer := &block.Footer{
			Version:     c.tableVersion,
			MetaIndexBH: metaBh,
		}
		footerBuf := footer.Serialise()
		if _, err := c.storageWriter.WriteRawBytes(footerBuf); err != nil {
			return err
		}
	}
	// Closes all buffers
	if err := c.storageWriter.Finish(); err != nil {
		return err
	}
	c.dataBlock = nil
	c.indexBlock = nil
	c.metaIndexBlock = nil

	return nil
}

func (c *ColBlockWriter) validate(key nogodb_common.InternalKey) error {
	if c.dataBlock.Rows() == 0 {
		return nil
	}

	prevKey := *c.dataBlock.CurrKey()
	cmp := c.comparer.Compare(key.UserKey, prevKey.UserKey)
	if cmp < 0 || (cmp == 0 && prevKey.Trailer < key.Trailer) {
		return fmt.Errorf("%w: keys must be added in strictly increasing order", common.ClientInvalidRequestError)
	}

	return nil
}

// doFlushWithoutLastKey flush the data block except the [currKey]
func (c *ColBlockWriter) doFlushWithoutLastKey(
	size int,
	prevKey *nogodb_common.InternalKey,
	currKey *nogodb_common.InternalKey,
) error {
	indexKey := &nogodb_common.InternalKey{
		UserKey: c.comparer.Separator(prevKey.UserKey, currKey.UserKey),
	}
	currRows := c.dataBlock.Rows()

	block.GrowSize(&c.uncompressed, size)

	c.uncompressed = c.dataBlock.Finish(currRows-1, size)

	task := block.SpawnNewTask()
	task.StorageWriter = c.storageWriter
	task.Physical = block.CompressToPb(
		c.compressors,
		c.checksumer,
		c.uncompressed,
	)
	task.IndexKey = indexKey
	task.IndexWriter = c.indexBlock

	c.taskQueue.Put(task)

	// reset the data block for the new writes
	c.dataBlock.Reset()
	if cap(c.uncompressed) > maxBlockRetainedSize {
		c.uncompressed = nil
	}

	return nil
}

func (c *ColBlockWriter) doFlushAll() error {
	currRows := c.dataBlock.Rows()
	size := int(c.dataBlock.Size())
	indexKey := &nogodb_common.InternalKey{
		UserKey: c.comparer.Successor(c.dataBlock.CurrKey().UserKey),
	}

	block.GrowSize(&c.uncompressed, size)

	c.uncompressed = c.dataBlock.Finish(currRows, size)

	task := block.SpawnNewTask()
	task.StorageWriter = c.storageWriter
	task.Physical = block.CompressToPb(
		c.compressors,
		c.checksumer,
		c.uncompressed,
	)
	task.IndexKey = indexKey
	task.IndexWriter = c.indexBlock

	c.taskQueue.Put(task)

	// reset the data block for the new writes
	c.dataBlock.Reset()
	if cap(c.uncompressed) > maxBlockRetainedSize {
		c.uncompressed = nil
	}

	return nil
}

func NewColBlockWriter(
	w go_fs.Writable,
	opts options.BlockWriteOpt,
	ver common.TableVersion,
) *ColBlockWriter {
	comparer := opts.Comparer
	storageWriter := storage.NewLayoutWriter(w)
	flushDecider := common.NewFlushDecider(opts.BlockSize, opts.BlockSizeThreshold)
	compressor := compression.NewCompressor(opts.DefaultCompression)
	checksumer := nogodb_common.NewChecksumer(nogodb_common.CRC32Checksum)

	return &ColBlockWriter{
		opt:          opts,
		tableVersion: ver,

		storageWriter: storageWriter,

		taskQueue: queue.NewQueue(flushQueueLen, false),

		filterWriter: filter.NewFilterWriter(filter.BloomFilter),

		flushDecider: flushDecider,
		comparer:     comparer,
		compressors:  compressor,
		checksumer:   checksumer,

		dataBlock: NewDataBlockWriter(comparer),
		indexBlock: NewIndexWriter(
			storageWriter,
			flushDecider,
			comparer,
			compressor,
			checksumer,
		),

		metaIndexBlock: NewKVBlockWriter(),
	}
}

var _ nogodb_common.InternalWriter = (*ColBlockWriter)(nil)
