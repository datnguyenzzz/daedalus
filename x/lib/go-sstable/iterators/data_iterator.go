package iterators

import (
	"errors"
	"fmt"
	"sync"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool/predictable_size"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/row_block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/common"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/filter"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/storage"
	"go.uber.org/zap"
)

// indexedIterator iter is the iteration of the data, received from the given index
type indexedIterator struct {
	nogodb_common.InternalIterator[nogodb_common.InternalKV]
	cmp nogodb_common.IComparer
	ver common.TableVersion

	reader    row_block.IBlockReader
	bpool     *predictable_size.PredictablePool
	index     *nogodb_common.InternalKV
	blockKind nogodb_common.BlockKind
}

func newIndexedIterator(
	cmp nogodb_common.IComparer,
	ver common.TableVersion,
	reader row_block.IBlockReader,
	bpool *predictable_size.PredictablePool,
	kind nogodb_common.BlockKind,
) *indexedIterator {
	return &indexedIterator{
		cmp:              cmp,
		ver:              ver,
		reader:           reader,
		bpool:            bpool,
		index:            nil,
		InternalIterator: nil,
		blockKind:        kind,
	}
}

func (ii *indexedIterator) SetIndexAndLoad(index *nogodb_common.InternalKV) error {
	if ii.index != nil && ii.index.Compare(ii.cmp, index) != 0 {
		if err := ii.Close(); err != nil {
			return err
		}
	}

	ii.index = index
	return ii.loadedIter()
}

// loadedIter ensure the iter is loaded from the current index.
// Usually it'd be called after the index updated
func (ii *indexedIterator) loadedIter() error {
	if ii.index == nil {
		return fmt.Errorf("%w: index is nil, can not load iterator", common.InternalServerError)
	}
	if ii.InternalIterator != nil {
		// iter is already loaded from the given current index
		return nil
	}

	bh := &common.BlockHandle{}
	if n := bh.DecodeFrom(ii.index.V.Value()); n <= 0 {
		zap.L().Error("failed to fully decode the index")
		return fmt.Errorf("%w: failed to fully decode the index", common.InternalServerError)
	}
	block, err := ii.reader.ReadThroughCache(bh, ii.blockKind)
	if err != nil {
		zap.L().Error("failed to read the block", zap.Error(err))
		return err
	}
	ii.InternalIterator = getBlockIter(ii.ver, ii.blockKind, ii.bpool, ii.cmp, block)
	return nil
}

func (ii *indexedIterator) Close() error {
	if ii.InternalIterator != nil {
		if err := ii.InternalIterator.Close(); err != nil {
			return err
		}
	}

	ii.InternalIterator = nil
	if ii.index != nil {
		ii.index.V.Release()
	}
	ii.index = nil
	return nil
}

// DataIterator reads the 2nd-level index block and creates and
// initializes an iterator over the 1st-level index, by which subsequently
// iterate over the datablock within the requested boundary [lower_bound, upper_bound]
//
// The index (1st + 2nd levels) have the last index, but not the first
// For example: (i+1-th key > index key ≥ i-th key )
// Data:    1 2 3     4 5 6    7 8 9    10 11
// L1(k,v):     [3,0]    [6,3]     [9,6]     [12,9]
// L2  :                    [6,0]                 [12,2]
//
// Todo (med) - Optimisation: Make all functions of DataIterator to be monotonic
type DataIterator struct {
	cmp         nogodb_common.IComparer
	blockReader row_block.IBlockReader
	ver         common.TableVersion

	bpool *predictable_size.PredictablePool
	// filter
	filterBH *common.BlockHandle
	filter   filter.IRead

	// the 2nd level index iterator do
	secondLevelIndexBH   *common.BlockHandle
	secondLevelIndexIter nogodb_common.InternalIterator[nogodb_common.InternalKV]

	// Iterators + indexes

	// Index: secondLevelIndex , Iter: firstLevelIndexIter
	firstLevelIndexedIter *indexedIterator
	// Index: firstLevelIndex , Iter: dataBlockIter
	dataIndexedIter *indexedIterator
}

var dataBlockIteratorPool = sync.Pool{
	New: func() any {
		return &DataIterator{}
	},
}

// SeekPrefixGTE the prefix is only used for checking with the bloom filter of the SSTable.
// but not used later while iterating. Hence, we can use the existing iterator position
// it did not fail bloom filter matching.
//
// key []byte is a full user key, aka internalKey.UserKey
func (i *DataIterator) SeekPrefixGTE(prefix, key []byte) *nogodb_common.InternalKV {
	// Refer to the col_block.writer, we only write UserKey[:prefix]
	// to the filter, without the MVCC suffix
	prefix = prefix[:i.cmp.Split(prefix)]
	if !i.filter.MayContain(prefix) {
		// don't invalidate the indexes and data block, the other iterator might still read it
		return nil
	}
	return i.SeekGTE(key)
}

// key []byte is a full user key, aka internalKey.UserKey
func (i *DataIterator) SeekGTE(key []byte) *nogodb_common.InternalKV {
	// Important notes:
	//  - Ensure the data (lazyValue) is released properly once the block is no longer used
	new2ndIndex := i.secondLevelIndexIter.SeekGTE(key)
	if new2ndIndex == nil {
		zap.L().Warn("the target key is not in the block")
		return nil
	}

	err := i.firstLevelIndexedIter.SetIndexAndLoad(new2ndIndex)
	if err != nil {
		zap.L().Error("failed to load the first level index", zap.Error(err))
		return nil
	}

	new1stIndex := i.firstLevelIndexedIter.SeekGTE(key)
	if new1stIndex == nil {
		panic("impossible, the 1st must be found")
	}

	err = i.dataIndexedIter.SetIndexAndLoad(new1stIndex)
	if err != nil {
		zap.L().Error("failed to load the data block", zap.Error(err))
		return nil
	}

	return i.dataIndexedIter.SeekGTE(key)
}

// key []byte is a full user key, aka internalKey.UserKey
func (i *DataIterator) SeekLTE(key []byte) *nogodb_common.InternalKV {
	// TODO implement me
	panic("implement me")
}

func (i *DataIterator) First() *nogodb_common.InternalKV {
	first2ndIndex := i.secondLevelIndexIter.First()
	if first2ndIndex == nil {
		panic("impossible, the first must be found on the 2nd level index")
	}

	err := i.firstLevelIndexedIter.SetIndexAndLoad(first2ndIndex)
	if err != nil {
		zap.L().Error("failed to load the first level index", zap.Error(err))
		return nil
	}

	first1stIndex := i.firstLevelIndexedIter.First()
	if first1stIndex == nil {
		panic("impossible, the first must be found on the 1st level index")
	}
	err = i.dataIndexedIter.SetIndexAndLoad(first1stIndex)
	if err != nil {
		zap.L().Error("failed to load the data block", zap.Error(err))
		return nil
	}

	return i.dataIndexedIter.First()
}

func (i *DataIterator) Last() *nogodb_common.InternalKV {
	// TODO implement me
	panic("implement me")
}

func (i *DataIterator) Next() *nogodb_common.InternalKV {
	if i.firstLevelIndexedIter == nil || i.dataIndexedIter == nil {
		zap.L().Error("the firstLevelIndexedIter and dataIndexedIter is nil. First will be returned")
		return i.First()
	}

	nextKv := i.dataIndexedIter.Next()
	if nextKv != nil {
		return nextKv
	}

	// the current data block is at the end, moving to the next block by the 1st index
	next1stIndex := i.firstLevelIndexedIter.Next()
	if next1stIndex != nil {
		err := i.dataIndexedIter.SetIndexAndLoad(next1stIndex)
		if err != nil {
			zap.L().Error("failed to load the data block", zap.Error(err))
			return nil
		}

		return i.dataIndexedIter.First()
	}

	// the current 1st level index is at the end, moving to the next 2nd index block
	next2ndIndex := i.secondLevelIndexIter.Next()
	if next2ndIndex == nil {
		return nil
	}

	err := i.firstLevelIndexedIter.SetIndexAndLoad(next2ndIndex)
	if err != nil {
		zap.L().Error("failed to load the first level index", zap.Error(err))
		return nil
	}
	err = i.dataIndexedIter.SetIndexAndLoad(i.firstLevelIndexedIter.First())
	if err != nil {
		zap.L().Error("failed to load the data block", zap.Error(err))
		return nil
	}

	return i.dataIndexedIter.First()
}

func (i *DataIterator) Prev() *nogodb_common.InternalKV {
	// TODO implement me
	panic("implement me")
}

func (i *DataIterator) Close() error {
	var err error
	if i.secondLevelIndexIter != nil {
		err = errors.Join(i.secondLevelIndexIter.Close())
	}
	if i.firstLevelIndexedIter != nil {
		err = errors.Join(i.firstLevelIndexedIter.Close())
	}
	if i.dataIndexedIter != nil {
		err = errors.Join(i.dataIndexedIter.Close())
	}
	i.blockReader.Release()
	dataBlockIteratorPool.Put(i)
	i.secondLevelIndexIter = nil
	i.firstLevelIndexedIter = nil
	i.dataIndexedIter = nil
	return err
}

func (i *DataIterator) IsClosed() bool {
	return i.secondLevelIndexIter == nil || i.secondLevelIndexIter.IsClosed()
}

func (i *DataIterator) readMetaIndexBlock(footer *block.Footer) error {
	// TODO(low): Should we cache the metaIndex block ?
	// Read and decode the meta index block
	metaIndexBuf, err := i.blockReader.Read(footer.GetMetaIndex(), nogodb_common.BlockKindMetaIntex)
	if err != nil {
		zap.L().Error("failed to read metaIndexBlock", zap.Error(err))
		return err
	}
	blkIter := getBlockIter(i.ver, nogodb_common.BlockKindMetaIntex, i.bpool, i.cmp, metaIndexBuf)
	for iter := blkIter.First(); iter != nil; iter = blkIter.Next() {
		val := iter.V.Value()
		bh := &common.BlockHandle{}
		if sz := bh.DecodeFrom(val); sz <= 0 {
			zap.L().Error("failed to decode block, corrupted size", zap.Any("block", i))
			return fmt.Errorf("failed to decode block, corrupted size. %w", common.InternalServerError)
		}

		switch iter.K.ReadMetaIndexKey() {
		case nogodb_common.BlockKindIndex:
			i.secondLevelIndexBH = bh
		case nogodb_common.BlockKindFilter:
			i.filterBH = bh
		default:
		}
	}

	return nil
}

func (i *DataIterator) init2ndLevelIndexBlockIterator() error {
	if i.secondLevelIndexBH == nil {
		zap.L().Error("the secondLevelIndex block handle is nil")
		return fmt.Errorf("%w: the secondLevelIndex block handle is nil", common.InternalServerError)
	}
	// TODO(low): Should we cache the 2nd level index block ?
	secondLevelIndexBuf, err := i.blockReader.Read(i.secondLevelIndexBH, nogodb_common.BlockKindIndex)
	if err != nil {
		zap.L().Error("failed to read secondLevelIndexBlock", zap.Error(err))
		return err
	}
	i.secondLevelIndexIter = getBlockIter(i.ver, nogodb_common.BlockKindIndex, i.bpool, i.cmp, secondLevelIndexBuf)
	return nil
}

func (i *DataIterator) readFilter() error {
	filterBlock, err := i.blockReader.ReadThroughCache(i.filterBH, nogodb_common.BlockKindFilter)
	if err != nil {
		zap.L().Error("failed to read filter", zap.Error(err))
		return err
	}

	i.filter = filter.NewFilterReader(filter.BloomFilter, filterBlock.Value())
	return nil
}

func getBlockIter(
	ver common.TableVersion,
	blockKind nogodb_common.BlockKind,
	bp *predictable_size.PredictablePool,
	cp nogodb_common.IComparer,
	data *nogodb_common.InternalLazyValue,
) nogodb_common.InternalIterator[nogodb_common.InternalKV] {
	if ver == common.TableV1 {
		return row_block.NewBlockIterator(bp, cp, data)
	}

	switch blockKind {
	case nogodb_common.BlockKindData:
		return col_block.NewDataBlockIter(bp, cp, data)
	case nogodb_common.BlockKindIndex:
		return col_block.NewIndexBlockIter(bp, cp, data)
	case nogodb_common.BlockKindMetaIntex:
		return col_block.NewKVBlockIter(bp, cp, data)
	default:
		panic(fmt.Sprintf("can not create iterator for block kind: %v", blockKind))
	}
}

func NewIterator(
	bpool *predictable_size.PredictablePool, // shared buffer pool across iterator
	fr go_fs.Readable,
	cmp nogodb_common.IComparer,
	opts *options.IteratorOpts,
) (*DataIterator, error) {
	iter := dataBlockIteratorPool.Get().(*DataIterator)
	var err error
	var footer *block.Footer
	var layoutReader storage.ILayoutReader

	iter.bpool = bpool
	iter.cmp = cmp
	layoutReader = storage.NewLayoutReader(fr)
	fullSize := fr.Size()
	footer, err = block.ReadFooter(layoutReader, fullSize)
	if err != nil {
		return nil, err
	}

	if iter.blockReader == nil {
		iter.blockReader = &row_block.RowBlockReader{}
	}

	iter.ver = footer.Version
	iter.blockReader.Init(iter.bpool, layoutReader, opts.CacheOpts)
	iter.firstLevelIndexedIter = newIndexedIterator(cmp, iter.ver, iter.blockReader, iter.bpool, nogodb_common.BlockKindIndex)
	iter.dataIndexedIter = newIndexedIterator(cmp, iter.ver, iter.blockReader, iter.bpool, nogodb_common.BlockKindData)

	if err = iter.readMetaIndexBlock(footer); err != nil {
		return nil, err
	}

	if err = iter.init2ndLevelIndexBlockIterator(); err != nil {
		return nil, err
	}

	if err = iter.readFilter(); err != nil {
		return nil, err
	}

	return iter, nil
}

var _ nogodb_common.InternalIterator[nogodb_common.InternalKV] = (*DataIterator)(nil)
