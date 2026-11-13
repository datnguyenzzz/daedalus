package options

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	go_block_cache "github.com/datnguyenzzz/nogodb/lib/go-block-cache"
	go_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
)

type IteratorOptsFunc func(opts *IteratorOpts)

type IteratorOpts struct {
	CacheOpts *CacheOptions
	Comparer  nogodb_common.IComparer
}

func WithComparer(comparer nogodb_common.IComparer) IteratorOptsFunc {
	return func(opts *IteratorOpts) {
		opts.Comparer = comparer
	}
}

func WithBlockCache(cache go_block_cache.IBlockCache, fd go_fs.FileDesc) IteratorOptsFunc {
	return func(opts *IteratorOpts) {
		opts.CacheOpts = &CacheOptions{}
		opts.CacheOpts.Cache = cache
		// notes: the block cache use file_num as a part of the cache key
		// a block cache with key = [file_num + offset], value = data[offset:offset + length]
		opts.CacheOpts.FileNum = go_fs.FromFileDescToFileNum(fd)
	}
}
