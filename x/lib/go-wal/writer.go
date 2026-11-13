package go_wal

import (
	"fmt"
	"io"
	"sync"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_record "github.com/datnguyenzzz/nogodb/lib/common/record"
	nogodb_fs "github.com/datnguyenzzz/nogodb/lib/go-fs"
)

// TODO(med): How to recycle the existing obsolete files, instead of re-creating
type WAL struct {
	opts options

	storager nogodb_fs.Storage

	// fileNum current created fileNum is openned for writing
	fileNum nogodb_common.DiskfileNum

	writer *writer

	mu struct {
		sync.Mutex
		// The queue of WALs, containing both flushed and unflushed WALs. The
		// flushed logs are a prefix, the unflushed logs a suffix.
		queue []nogodb_common.DiskfileNum
	}
}

func NewWalManager(dir string, opts ...OptionFn) (*WAL, error) {
	w := &WAL{
		fileNum: -1,
	}
	for _, o := range opts {
		o(w)
	}
	var err error
	w.storager, err = nogodb_fs.OpenVfsProvider(
		nogodb_fs.WithDirName(dir),
		nogodb_fs.WithBytesPerSync(int64(w.opts.bytesPerSync)),
		nogodb_fs.WithFS(w.opts.fs),
	)
	if err != nil {
		return nil, err
	}

	return w, nil
}

// List returns the WALs info in ascending order of file name.
// List does not perform I/O
func (w *WAL) List() []nogodb_common.DiskfileNum {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.mu.queue
}

// Obsolete informs the manager that all WALs less than minUnflushedNum are obsolete.
func (w *WAL) Obsolete(minUnflushedNum nogodb_common.DiskfileNum) (toDelete []nogodb_common.DiskfileNum, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	start := len(w.mu.queue)
	for i, fn := range w.mu.queue {
		if fn >= minUnflushedNum {
			start = i
			break
		}

		toDelete = append(toDelete, fn)
	}

	w.mu.queue = w.mu.queue[start:]

	return toDelete, nil
}

// Create creates a new WAL. NumWALs passed to successive Create calls must be
// monotonically increasing, and be greater than any NumWAL seen earlier. The
// caller must close the previous Writer before calling Create.
func (w *WAL) Create(fileNum nogodb_common.DiskfileNum) (io.WriteCloser, error) {
	if fileNum <= w.fileNum {
		return nil, fmt.Errorf("the requested fileNum must be monotonically increasing, last value: %d", w.fileNum)
	}

	wf, _, err := w.storager.Create(nogodb_common.TypeWAL, fileNum)
	if err != nil {
		return nil, err
	}

	if err := w.storager.Sync(nogodb_common.TypeWAL, fileNum); err != nil {
		return nil, err
	}
	// TODO(low): wf is buffered to the RAM before flushing, is it ok ?
	wr := nogodb_record.NewWriter(wf)
	w.writer = &writer{wr}
	w.mu.Lock()
	defer w.mu.Unlock()

	w.mu.queue = append(w.mu.queue, fileNum)

	return w.writer, nil
}

type writer struct {
	wr *nogodb_record.Writer
}

func (w *writer) Write(p []byte) (n int, err error) {
	next, err := w.wr.Next()
	if err != nil {
		return n, err
	}

	return next.Write(p)
}

func (w *writer) Close() error {
	return w.wr.Close()
}

func (w *WAL) Close() error {
	err := w.writer.Close()
	w.writer = nil
	return err
}

var _ IWalWriter = (*WAL)(nil)
