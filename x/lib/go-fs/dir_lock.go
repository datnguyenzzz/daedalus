package go_fs

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

type DirLockSet struct {
	acquired []*DirLock
}

func (d *DirLockSet) Close() error {
	var err error
	for _, l := range d.acquired {
		err = errors.Join(err, l.Close())
	}
	d.acquired = nil
	return err
}

func (d *DirLockSet) Acquire(dirName string, fs FS) (*DirLock, error) {
	l, err := LockDir(dirName, fs)
	if err != nil {
		return nil, err
	}
	d.acquired = append(d.acquired, l)
	return l, err
}

// DirLock represents a file lock on a directory
type DirLock struct {
	dirname  string
	fileLock io.Closer
	// refs is a count of the number of handles on the lock. refs must be 0, 1
	// or 2.
	//
	// When acquired by the client and passed to Open, refs = 1 and the Open
	// call increments it to 2. When the database is closed, it's decremented to
	// 1. Finally when the original caller calls Close on the Lock, it's
	// decremented to zero and the underlying file lock is released.
	//
	// When Open acquires the file lock, refs remains at 1 until the database is
	// closed.
	refs atomic.Int32
}

func LockDir(dirName string, fs FS) (*DirLock, error) {
	path := fs.PathJoin(dirName, nogodb_common.ObjectTypeToString[nogodb_common.TypeLock])

	fileLock, err := fs.Lock(path)
	if err != nil {
		return nil, err
	}

	dl := &DirLock{
		dirname:  dirName,
		fileLock: fileLock,
	}

	dl.refs.Store(1)

	return dl, err
}

func (l *DirLock) tryToOpen() error { //nolint: unused
	// During Open, when a user passed in a lock, the reference count must be
	// exactly 1. If it's zero, the lock is no longer held and is invalid. If
	// it's 2, the lock is already in use by another database within the
	// process.
	if !l.refs.CompareAndSwap(1, 2) {
		return fmt.Errorf("pebble: unexpected %q DirLock reference count; is the lock already in use?", l.dirname)
	}
	return nil
}

// Close releases the lock, permitting another process to lock and open the
// database. Close must not be called until after a database using the Lock has
// been closed.
func (l *DirLock) Close() error {
	v := l.refs.Add(-1)
	if v > 0 {
		return nil
	} else if v < 0 {
		return fmt.Errorf("pebble: unexpected %q DirLock reference count %d", l.dirname, v)
	}
	defer func() { l.fileLock = nil }()
	return l.fileLock.Close()
}
