package options

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	go_block_cache "github.com/datnguyenzzz/nogodb/lib/go-block-cache"
)

type CacheOptions struct {
	Cache   go_block_cache.IBlockCache
	FileNum nogodb_common.DiskfileNum
}
