package go_blocked_bloom_filter

import (
	"encoding/binary"
)

const (
	defaultBitsPerKeys = 10
	blockBytesSize     = 64                 // Fit 1 CPU cache line
	blockBitsSize      = 8 * blockBytesSize // Fit 1 CPU cache line
)

// bloomFilter is an implementation of the blocked Bloom Filter with the bit patterns
// https://save-buffer.github.io/bloom_filter.html
type (
	bloomFilter       struct{}
	bloomFilterWriter struct {
		// bitsPerKeys = <bits size of entire of an entire array> / <number of the keys>
		// For the given eps (the desired false positive rate),
		//   - The minimum of bitsPerKeys = log(2)*eps
		//   - With the corresponding number of hash functions (probe), k = log(2)*eps
		//
		// Simply the equation backwards, we get: eps = bitPerKeys / log(2)
		// Therefore, there are diminishing returns on eps past around bitsPerKeys = 10 (we chose 10 as defaultBitsPerKeys)
		bitsPerKeys int

		hashes  []uint32
		numKeys int
	}
)

// Writer \\

func (bw *bloomFilterWriter) Add(key []byte) {
	h := bloomHash(key)
	bw.hashes = append(bw.hashes, h)
	bw.numKeys++
}

func (bw *bloomFilterWriter) Build(b *[]byte) {
	var nBlocks int
	var nProbes byte
	// Layout
	// Bytes form: |  b1, b2, ,,,, bn |  b1, b2, ,,,, bn | ,,,,
	//             | block 1 64 bytes | block 1 64 bytes | ....
	// Bits form:  |    000...0000    |    000...0000    | ....
	//             | block 1 512 bits | block 2 512 bits | ....
	//
	// 1. calculate number of cache lines to fit all the added keys (round up).
	// Each block holds maximum of 64 bytes (1 CPU cache line)
	nBlocks = (bw.numKeys*bw.bitsPerKeys + blockBitsSize - 1) / blockBitsSize
	// Make nBlocks an odd number to make sure more bits are involved when
	// determining which block.
	if nBlocks%2 == 0 {
		nBlocks++
	}
	nBytes := nBlocks * blockBytesSize

	// freeSpaces points to the starting index of the "b" buffer that are free to write
	var freeSpaces []byte

	// 2. grow the given buffer to have at least spaces to hold the data
	// [nBytes (for all the hashes) + 4 (for number of lines) + 1 (for number of probes)] bytes
	wantSize := nBytes + 5 + len(*b)
	if wantSize <= cap(*b) {
		*b = (*b)[:wantSize]
		freeSpaces = (*b)[len(*b):]
		clear(freeSpaces)
	} else {
		// grow (exponentially) the given buffer to have enough needed spaces
		neededSize := 64
		for neededSize < wantSize {
			neededSize += neededSize / 4
		}

		tmp := *b
		*b = make([]byte, wantSize, neededSize)
		freeSpaces = (*b)[len(tmp):]
		copy(*b, tmp)
	}

	// 3. build the filters
	nProbes = calculateProbes(bw.bitsPerKeys)
	for _, h := range bw.hashes {
		delta := h>>17 | h<<15
		// 3.1 Each key maps to one block (line)
		b := (h % uint32(nBlocks)) * blockBitsSize
		// 3.2 Generate the bit pattern that have exactly `nProbes` bits are 1
		for p := byte(0); p < nProbes; p++ {
			bitPos := b + (h % blockBitsSize)
			byteIdx, bitIdx := bitPos/8, bitPos%8
			freeSpaces[byteIdx] |= 1 << bitIdx
			h += delta
		}
	}
	freeSpaces[nBytes] = nProbes
	binary.LittleEndian.PutUint32(freeSpaces[nBytes+1:], uint32(nBlocks))

	// 4. Release
	bw.hashes = bw.hashes[:0]
	bw.numKeys = 0
}

// End of Writer \\

func (bf *bloomFilter) NewWriter() IWriter {
	return &bloomFilterWriter{
		bitsPerKeys: defaultBitsPerKeys,
		hashes:      []uint32{},
	}
}

func (bf *bloomFilter) MayContain(filter, key []byte) bool {
	// required at least 5 bytes for the nLines and nProbes
	if len(filter) <= 5 {
		return false
	}
	n := len(filter) - 5
	nProbes := filter[n]
	nBlocks := binary.LittleEndian.Uint32(filter[n+1:])
	cacheLineBits := 8 * (uint32(n) / nBlocks)

	// Check if block contains pattern bits
	h := bloomHash(key)
	delta := h>>17 | h<<15
	// 1. Get a block of the given key
	b := (h % nBlocks) * cacheLineBits
	// 2. The key is considered to be membership, if a block contains all the bits pattern of the key
	// Can refer to the Build() function for the detail how the pit pattern is generated
	for j := byte(0); j < nProbes; j++ {
		bitPos := b + (h % cacheLineBits)
		byteIdx := bitPos / 8
		if filter[byteIdx]&(1<<(bitPos%8)) == 0 {
			return false
		}
		h += delta
	}
	return true
}

func (bf *bloomFilter) Name() string {
	return "nogodb.go_blocked_bloom_filter.BloomFilter"
}

func NewBloomFilter() IFilter {
	return &bloomFilter{}
}

func calculateProbes(bitsPerKey int) byte {
	n := byte(float64(bitsPerKey) * 0.69) // 0.69 =~ ln(2)
	if n < 1 {
		n = 1
	}
	if n > 30 {
		n = 30
	}
	return n
}

// bloomHash return hash of the given data. Similar to murmur hash function
func bloomHash(b []byte) uint32 {
	const (
		seed = 0xbc9f1d34
		m    = 0xc6a4a793
	)
	h := uint32(seed) ^ uint32(uint64(uint32(len(b))*m))
	for ; len(b) >= 4; b = b[4:] {
		h += uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		h *= m
		h ^= h >> 16
	}

	switch len(b) {
	case 3:
		h += uint32(int8(b[2])) << 16
		fallthrough
	case 2:
		h += uint32(int8(b[1])) << 8
		fallthrough
	case 1:
		h += uint32(int8(b[0]))
		h *= m
		h ^= h >> 24
	}
	return h
}

var (
	_ IFilter = (*bloomFilter)(nil)
	_ IWriter = (*bloomFilterWriter)(nil)
)
