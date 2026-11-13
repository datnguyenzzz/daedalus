package common

type BlockKind byte

const (
	BlockKindUnknown BlockKind = iota
	BlockKindData
	BlockKindIndex
	BlockKindFilter
	BlockKindMetaIntex
)

var BlockKindStrings = map[BlockKind]string{
	BlockKindData:      "data",
	BlockKindIndex:     "index",
	BlockKindFilter:    "filter",
	BlockKindMetaIntex: "meta-index",
}
