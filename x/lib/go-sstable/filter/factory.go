package filter

import "github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter"

type Method byte

const (
	Unknown Method = iota
	BloomFilter
	RibbonFilter
)

type IRead interface {
	Name() string
	// MayContain returns whether the encoded filter may contain given key.
	// False positives are possible, where it returns true for keys not in the
	// original set.
	MayContain(key []byte) bool
}

type IWriter interface {
	// Add adds a key to the filter generator.
	Add(key []byte)
	// Build generates encoded filters based on keys passed so far.
	Build(pb *[]byte)
}

func NewFilterWriter(method Method) IWriter {
	switch method {
	case BloomFilter:
		bf := go_blocked_bloom_filter.NewBloomFilter()
		return bf.NewWriter()
	default:
		panic("unsupported / unknown filtering method")
	}
}

type Reader struct {
	bl     go_blocked_bloom_filter.IFilter
	filter []byte
}

func (r *Reader) Name() string {
	return r.bl.Name()
}

func (r *Reader) MayContain(key []byte) bool {
	return r.bl.MayContain(r.filter, key)
}

// NewFilterReader Caller should build the filter first by using the IWriter
// the pass the built filter into the reader
func NewFilterReader(method Method, filter []byte) IRead {
	switch method {
	case BloomFilter:
		return &Reader{bl: go_blocked_bloom_filter.NewBloomFilter(), filter: filter}
	default:
		panic("unsupported / unknown filtering method")
	}
}

var _ IRead = (*Reader)(nil)
