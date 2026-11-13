package common

import "encoding/binary"

const (
	MagicNumber = "\x6e\x6f\x67\x6f\x64\x62\x6b\x76"
)

type TableVersion byte

const (
	TableV1 TableVersion = iota // Row block table
	TableV2                     // Column block table
)

const (
	TableVersionLen = 4
	MagicNumberLen  = 8
)

var TableFooterSize = map[TableVersion]int{
	TableV1: binary.MaxVarintLen64 + TableVersionLen + MagicNumberLen,
	TableV2: binary.MaxVarintLen64 + TableVersionLen + MagicNumberLen,
}

var MaxPossibleFooterSize = TableFooterSize[TableV1]
