package prefixbytescodex

import (
	"bytes"
	"slices"

	"github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex"
	rawbytescodex "github.com/datnguyenzzz/nogodb/lib/go-sstable/block/col_block/codex/raw_bytes_codex"
)

type PrefixBytesDecoder struct {
	rows       uint32
	bundleSize byte

	rawBytesDec *rawbytescodex.RawBytesDecoder
}

func (u *PrefixBytesDecoder) Get(row uint32) []byte {
	bundlePrefixPos := GetBundlePrefixPos(row, u.bundleSize)
	bundlePrefix := u.rawBytesDec.Get(bundlePrefixPos)
	suffix := u.rawBytesDec.Get(GetOffsetFromRow(row, u.bundleSize))

	return slices.Concat(u.rawBytesDec.Get(0), bundlePrefix, suffix)
}

func (e *PrefixBytesDecoder) DataType() codex.DataType {
	return codex.PrefixCompressedBytesDT
}

// SeekGTE moves the iterator to the first key/value pair whose key ≥ to the given key.
// Only PrefixBytesDecoder can support this function because the keys are sorted.
//
// NOTE: [0, rows-1] is used instead of [from, to]
func (u *PrefixBytesDecoder) SeekGTE(key []byte, from, to int32) (rowIndex uint32, isEqual bool) {
	blockPrefix := u.rawBytesDec.Get(0)

	if len(key) < len(blockPrefix) {
		return 0, false
	}

	if len(blockPrefix) > 0 {
		switch bytes.Compare(key[:len(blockPrefix)], blockPrefix) {
		case 1:
			return u.rows, false
		case -1:
			return 0, false
		}
	}

	key = key[len(blockPrefix):]

	// Binary search to find the right bundle
	lo, hi := 0, int(GetBundleFromRow(u.rows-1, u.bundleSize))
	// bundle is the largest bundle that < key
	bundle := -1
	for lo <= hi {
		mid := (lo + hi) >> 1
		offset := GetBundleStartOffset(uint32(mid), u.bundleSize)
		firstKey := u.rawBytesDec.Slice(offset, offset+1)
		cp := bytes.Compare(firstKey, key)
		if cp < 0 {
			bundle = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	if bundle == -1 {
		// all keys are ≥ given [key]
		key0 := u.rawBytesDec.Slice(1, 2)
		return 0, bytes.Equal(key, key0)
	}

	bundlePrefix := u.rawBytesDec.Get(GetBundleStartOffset(uint32(bundle), u.bundleSize))

	if len(key) < len(bundlePrefix) || !bytes.Equal(key[:len(bundlePrefix)], bundlePrefix) {
		// the founded is the first key of next bundle
		if uint32(bundle) == GetBundleFromRow(u.rows-1, u.bundleSize) {
			// key is greater than all keys in the block
			return u.rows, false
		}

		offset := GetBundleStartOffset(uint32(bundle+1), u.bundleSize)
		firstKey := u.rawBytesDec.Slice(offset, offset+1)

		return GetRowFromOffset(offset+1, u.bundleSize), bytes.Equal(key, firstKey)
	}

	// Binary search to find the index on the bundle
	keyPrefix := key[:len(bundlePrefix)]
	key = key[len(bundlePrefix):]
	lo = int(GetBundleStartOffset(uint32(bundle), u.bundleSize) + 1)
	hi = int(min(GetBundleStartOffset(uint32(bundle+1), u.bundleSize)-1, GetOffsetFromRow(u.rows-1, u.bundleSize)))
	// fmt.Println("range:", lo, hi)
	cpResult := 2
	for lo <= hi {
		mid := (lo + hi) >> 1
		cp := bytes.Compare(u.rawBytesDec.Get(uint32(mid)), key)
		if cp >= 0 {
			rowIndex = GetRowFromOffset(uint32(mid), u.bundleSize)
			cpResult = cp
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	if cpResult == 2 {
		// the key must be either not found if it is the last bundle
		// or the first key of the next bundle
		if uint32(bundle) == GetBundleFromRow(u.rows-1, u.bundleSize) {
			// key is greater than all keys in the block
			return u.rows, false
		}

		offset := GetBundleStartOffset(uint32(bundle+1), u.bundleSize)
		firstKey := u.rawBytesDec.Slice(offset, offset+1)
		key = slices.Concat(keyPrefix, key)

		return GetRowFromOffset(offset+1, u.bundleSize), bytes.Equal(key, firstKey)
	}

	return rowIndex, cpResult == 0
}

func (u *PrefixBytesDecoder) Slice(from, to uint32) []byte {
	panic("Not yet implemented")
}

func (u *PrefixBytesDecoder) Rows() uint32 {
	return u.rows
}

func NewPrefixBytesDecoder(
	rows, offset uint32,
	data []byte,
) (codex.IColumnDecoder[[]byte], uint32) {
	dec := &PrefixBytesDecoder{rows: rows}

	dec.bundleSize = data[offset]

	rawSize := GetOffsetFromRow(rows-1, dec.bundleSize) + 1

	d, newOffset := rawbytescodex.NewRawBytesDecoder(
		rawSize, offset+1, data, // skip 1 byte for bundle size
	)

	var ok bool
	dec.rawBytesDec, ok = d.(*rawbytescodex.RawBytesDecoder)
	if !ok {
		panic("NewPrefixBytesDecoder, fail to assert to RawBytesDecoder")
	}

	return dec, newOffset
}

var _ codex.IColumnDecoder[[]byte] = (*PrefixBytesDecoder)(nil)
