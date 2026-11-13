package go_wal

import (
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
)

type OptionFn func(*WAL)

type options struct {
	fs nogodb_fs.FS

	// bytesPerSync specifies the number of bytes to write before calling sync.
	bytesPerSync uint32

	logger nogodb_common.Logger
}

func WithBytesPerSync(bytesPerSync uint32) OptionFn {
	return func(wal *WAL) {
		wal.opts.bytesPerSync = bytesPerSync
	}
}

func WithFS(fs nogodb_fs.FS) OptionFn {
	return func(w *WAL) {
		w.opts.fs = fs
	}
}

func WithLogger(l nogodb_common.Logger) OptionFn {
	return func(w *WAL) {
		w.opts.logger = l
	}
}
