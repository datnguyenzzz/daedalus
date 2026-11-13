package go_block_cache

import (
	"encoding/binary"

	"github.com/twmb/murmur3"
)

func murmur32(ns, key uint64) uint32 {
	buf := make([]byte, 16)

	binary.LittleEndian.PutUint64(buf[0:8], ns)
	binary.LittleEndian.PutUint64(buf[8:16], key)

	return murmur3.Sum32(buf)
}

func computeSize(v Value) int {
	return cap(v)
}
