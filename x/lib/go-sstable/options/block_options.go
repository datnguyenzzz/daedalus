package options

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
)

type CompressionOpts map[nogodb_common.BlockKind]compression.CompressionType

type BlockWriteOpt struct {
	// BlockRestartInterval is the number of keys between restart points for delta encoding of keys.
	//
	// The default value is 16.
	BlockRestartInterval int

	// BlockSize is the target uncompressed size in bytes of each table block.
	//
	// The default value is 4KB.
	BlockSize int

	// BlockSizeThreshold finish/close a block if the block size is larger than the
	// specified percentage of the target block size and adding the next entry
	// would cause the block to be larger than the target block size.
	//
	// The default value is 0.9.
	BlockSizeThreshold float32

	Comparer nogodb_common.IComparer

	// Compression defines the per-block compression to use.
	Compression CompressionOpts
	// DefaultCompression In case the block doesn't have a specified compression to use,
	// the algorithm defined by DefaultCompression will be chosen
	DefaultCompression compression.CompressionType
}
