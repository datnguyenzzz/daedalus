package prefixbytescodex

import (
	"fmt"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
)

type PrefixBytesEncoder struct {
	rows uint32

	bundlePrefix []byte
	// bundlePrefixBefore is the bundle prefix before adding the current key
	bundlePrefixBefore []byte

	// values is the concatenated of values, without prefix compression
	values        [][]byte
	valuesEncoder *rawbytescodex.RawByteEncoder

	// bundle
	BundleSize byte
}

func (e *PrefixBytesEncoder) Init() {
	if e.BundleSize <= 0 {
		panic("PrefixBytesEncoder must have bundleSize > 0")
	}
	if e.BundleSize&(e.BundleSize-1) != 0 {
		panic("PrefixBytesEncoder must have bundleSize of 2^x")
	}

	e.valuesEncoder = new(rawbytescodex.RawByteEncoder)

	e.Reset()
}

// Reset reuses the existing encoder with its already allocated memory
func (e *PrefixBytesEncoder) Reset() {
	e.rows = 0
	e.bundlePrefix = nil

	e.values = e.values[:0]
	e.values = append(e.values, nil) // reserve 1 slot for the block prefix
	e.valuesEncoder.Reset()
}

func (e *PrefixBytesEncoder) Append(v []byte) {
	if e.rows%uint32(e.BundleSize) == 0 {
		e.bundlePrefix = v
		e.bundlePrefixBefore = nil
		e.values = append(e.values, nil) // reserve 1 slot for the bundle prefix
	} else {
		lcp := block.CommonPrefix(e.bundlePrefix, v)
		e.bundlePrefixBefore = e.bundlePrefix
		e.bundlePrefix = e.bundlePrefix[:lcp]
	}

	e.rows += 1
	e.values = append(e.values, v)

	if e.rows%uint32(e.BundleSize) == 0 {
		e.compressBundle()
	}
}

func (e *PrefixBytesEncoder) compressBundle() {
	bundlePrefixPos := GetBundlePrefixPos(e.rows-1, e.BundleSize)
	e.values[bundlePrefixPos] = e.bundlePrefix

	for i := int(bundlePrefixPos) + 1; i < len(e.values); i++ {
		e.values[i] = e.values[i][len(e.bundlePrefix):]
	}
}

// Size returns the "estimated" size of the column,
// if the its row were encoded starting from an [offset]
// The "estimated" size will always be greater than the actual size after encoded
func (e *PrefixBytesEncoder) Size(offset uint32) uint32 {
	var totalLen uint32
	for _, v := range e.values {
		totalLen += uint32(len(v))
	}

	// account for the last bundle that has not yet compressed
	if e.rows%uint32(e.BundleSize) != 0 {
		bundlePrefixPos := GetBundlePrefixPos(e.rows-1, e.BundleSize)
		totalLen += uint32(len(e.bundlePrefix))
		totalLen -= max(0, uint32(len(e.values))-bundlePrefixPos-1) * uint32(len(e.bundlePrefix))
	}

	var blockPrefix []byte = e.bundlePrefix
	for i := 0; i < len(e.values); i += int(e.BundleSize) + 1 {
		if len(e.values[i]) == 0 {
			continue
		}
		lcp := block.CommonPrefix(blockPrefix, e.values[i])
		blockPrefix = blockPrefix[:lcp]
	}

	// adjust the bundle prefix
	totalLen -= uint32((len(e.values)+int(e.BundleSize))/int(e.BundleSize+1)) * uint32(len(blockPrefix))

	return (1 +
		/* block prefix len*/ uint32(len(blockPrefix)) +
		totalLen +
		1 + 8 +
		/* 4-byte per offsets */ uint32(len(e.values)*4))
}

func (e *PrefixBytesEncoder) DataType() codex.DataType {
	return codex.PrefixCompressedBytesDT
}

// Finish serialises the encoded column into a [buf] from [offset], return the offset after written
func (e *PrefixBytesEncoder) Finish(rows, offset uint32, buf []byte) uint32 {
	if rows < e.rows-1 || rows > e.rows {
		panic(fmt.Sprintf("PrefixBytesEncoder only accepts to finish either all rows, or [all rows minus 1], %d >< %d", rows, e.rows))
	}

	// compress the unfinished bundle
	if e.rows%uint32(e.BundleSize) != 0 {
		e.compressBundle()
	}

	end := len(e.values)
	var needRecompute bool
	if rows == e.rows-1 {
		needRecompute = true
		end -= 1
		if e.rows%uint32(e.BundleSize) == 1 {
			// remove the bundle prefix
			end -= 1
			needRecompute = false
		}
	}

	e.rows = rows
	e.values = e.values[:end]
	if needRecompute {
		extraPrefix := len(e.bundlePrefixBefore) - len(e.bundlePrefix)
		bundlePrefixPos := GetBundlePrefixPos(e.rows-1, e.BundleSize)
		e.values[bundlePrefixPos] = e.bundlePrefixBefore

		for i := int(bundlePrefixPos) + 1; i < len(e.values); i++ {
			e.values[i] = e.values[i][extraPrefix:]
		}
	}

	// find the block prefix
	var blockPrefix []byte = nil
	for i := 1; i < len(e.values); i += int(e.BundleSize) + 1 {
		if blockPrefix == nil {
			blockPrefix = e.values[i]
			continue
		}

		lcp := block.CommonPrefix(blockPrefix, e.values[i])
		blockPrefix = blockPrefix[:lcp]
	}
	e.values[0] = blockPrefix

	// adjust the bundle prefix
	for i := 1; i < len(e.values); i += int(e.BundleSize) + 1 {
		e.values[i] = e.values[i][len(blockPrefix):]
	}

	// start encoding into the [buf]
	// refer to the colblock/README.md for more detail about the layout
	buf[offset] = e.BundleSize
	offset++
	for _, v := range e.values {
		e.valuesEncoder.Append(v)
	}

	// fmt.Println("PrefixBytes", "enc.size=", e.valuesEncoder.Size(offset))
	offset = e.valuesEncoder.Finish(uint32(len(e.values)), offset, buf)

	return offset
}

var _ codex.IColumnEncoder[[]byte] = (*PrefixBytesEncoder)(nil)
