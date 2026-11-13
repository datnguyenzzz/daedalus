package go_fs

import (
	"bufio"
	"sync/atomic"
)

type bufferFileWritable struct {
	File
	bw *bufio.Writer
}

var _ Writable = (*bufferFileWritable)(nil)

func NewBufferedFileWriable(f File) *bufferFileWritable {
	return &bufferFileWritable{
		File: f,
		bw:   bufio.NewWriter(f),
	}
}

func (f *bufferFileWritable) Write(p []byte) (n int, err error) {
	return f.bw.Write(p)
}

func (f *bufferFileWritable) Sync() error {
	if err := f.bw.Flush(); err != nil {
		return err
	}

	if err := f.File.Sync(); err != nil {
		return err
	}

	return nil
}

func (f *bufferFileWritable) Finish() error {
	_ = f.bw.Flush()
	err := f.File.Close()
	f.bw = nil
	f.File = nil
	return err
}

func (f *bufferFileWritable) Abort() {
	_ = f.File.Close()
}

// syncableFile wraps a writable file and ensures that data is synced
// periodically as it is written.
type syncableFile struct {
	File
	bytesPerSync int64
	// The offset at which dirty data has been written.
	offset atomic.Int64
	// The offset at which data has been synced.
	syncedOffset atomic.Int64
}

func NewSyncableFile(file File, bytesPerSync int64) *syncableFile {
	f := &syncableFile{
		File:         file,
		bytesPerSync: bytesPerSync,
	}

	f.syncedOffset.Store(-1)
	return f
}

func (f *syncableFile) Write(p []byte) (n int, err error) {
	n, err = f.File.Write(p)
	if err != nil {
		return n, err
	}

	f.offset.Add(int64(n))
	if err = f.maySync(); err != nil {
		return 0, err
	}

	return n, nil
}

func (f *syncableFile) Close() error {
	if f.offset.Load() > f.syncedOffset.Load() {
		if err := f.File.Sync(); err != nil {
			return err
		}
	}

	return f.File.Close()
}

func (f *syncableFile) maySync() error {
	if f.bytesPerSync <= 0 {
		return nil
	}

	offset := f.offset.Load()
	syncedOffset := f.syncedOffset.Load()
	if offset-syncedOffset < f.bytesPerSync {
		return nil
	}

	if err := f.File.Sync(); err != nil {
		return err
	}

	f.updateSyncedOffset(offset)

	return nil
}

func (f *syncableFile) updateSyncedOffset(offset int64) {
	for {
		syncedOffset := f.syncedOffset.Load()
		if syncedOffset >= offset {
			return
		}
		if f.syncedOffset.CompareAndSwap(syncedOffset, offset) {
			return
		}
	}
}
