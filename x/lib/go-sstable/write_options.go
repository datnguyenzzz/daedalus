package go_sstable

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/datnguyenzzz/nogodb/lib/common/compression"
	"github.com/datnguyenzzz/nogodb/lib/go-sstable/options"
)

type WriteOptFn func(w *Writer)

var DefaultWriteOpt = &options.BlockWriteOpt{
	BlockRestartInterval: 16,
	BlockSize:            4 * 1024,
	BlockSizeThreshold:   0.9,
	DefaultCompression:   compression.SnappyCompression,
	Comparer:             nogodb_common.NewComparer(),
}

func WithComparer(cp nogodb_common.IComparer) WriteOptFn {
	return func(w *Writer) {
		w.datablockOpts.Comparer = cp
	}
}

func WithBlockRestartInterval(interval int) WriteOptFn {
	return func(w *Writer) {
		w.datablockOpts.BlockRestartInterval = interval
	}
}

func WithBlockSize(blockSize int) WriteOptFn {
	return func(w *Writer) {
		w.datablockOpts.BlockSize = blockSize
	}
}

func WithBlockSizeThreshold(blockSizeThreshold float32) WriteOptFn {
	return func(w *Writer) {
		w.datablockOpts.BlockSizeThreshold = blockSizeThreshold
	}
}

func WithCompression(compressor compression.CompressionType) WriteOptFn {
	return func(w *Writer) {
		w.datablockOpts.DefaultCompression = compressor
	}
}
