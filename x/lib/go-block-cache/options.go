package go_block_cache

type CacheOpt func(c *hashMap)

func WithShardNum(shardNum int) CacheOpt {
	return func(c *hashMap) {
		if shardNum > 0 {
			c.shardNum = shardNum
		}
	}
}

func WithMaxSize(maxSize int64) CacheOpt {
	return func(c *hashMap) {
		if maxSize > 0 {
			c.maxSize = maxSize
		}
	}
}

func WithCacheType(cacheType CacheType) CacheOpt {
	return func(c *hashMap) {
		c.cacheType = cacheType
	}
}
