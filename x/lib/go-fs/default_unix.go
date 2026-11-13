// TODO(low) control by go build tags
package go_fs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"golang.org/x/sys/unix"
)

type unixFile struct {
	*os.File
}

func (f *unixFile) Stat() (FileInfo, error) {
	return f.File.Stat()
}

type defaultUnix struct{}

func NewDefaultUnix() *defaultUnix {
	return &defaultUnix{}
}

func wrapOSFile(f *os.File) unixFile {
	return unixFile{File: f}
}

func (f *defaultUnix) OpenDir(name string) (File, error) {
	file, err := os.OpenFile(name, syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	return wrapOSFile(file), nil
}

func (f *defaultUnix) Create(name string, objType nogodb_common.ObjectType) (File, error) {
	const openFlags = os.O_RDWR | os.O_CREATE | os.O_EXCL | syscall.O_CLOEXEC
	osFile, err := os.OpenFile(name, openFlags, 0o666)
	// If the file already exists, remove it and try again.
	if os.IsNotExist(err) {
		if err = f.Remove(name); err != nil {
			return nil, err
		}

		osFile, err = os.OpenFile(name, openFlags, 0o666)
	}

	if err != nil {
		return nil, err
	}

	return wrapOSFile(osFile), nil
}

func (f *defaultUnix) Open(name string) (File, error) {
	osFile, err := os.OpenFile(name, os.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	return wrapOSFile(osFile), nil
}

func (f *defaultUnix) Remove(name string) error {
	return os.Remove(name)
}

func (f *defaultUnix) List(dir string) ([]string, error) {
	osFile, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer osFile.Close()

	return osFile.Readdirnames(-1)
}

func (f *defaultUnix) PathBase(path string) string {
	return filepath.Base(path)
}

func (f *defaultUnix) Stat(name string) (FileInfo, error) {
	return os.Stat(name)
}

func (f *defaultUnix) PathJoin(elem ...string) string {
	return filepath.Join(elem...)
}

func (f *defaultUnix) MkdirAll(dir string, perm os.FileMode) error {
	return os.MkdirAll(dir, perm)
}

func (f *defaultUnix) PathDir(path string) string {
	return filepath.Dir(path)
}

var lockedFile struct {
	mu    sync.Mutex
	files map[string]bool
}

type lockCloser struct {
	name string
	f    *os.File
}

func (l lockCloser) Close() error {
	lockedFile.mu.Lock()
	defer lockedFile.mu.Unlock()
	if _, ok := lockedFile.files[l.name]; !ok {
		panic(fmt.Errorf("lock file %q is not locked", l.name))
	}
	delete(lockedFile.files, l.name)
	return l.f.Close()
}

func (f *defaultUnix) Lock(name string) (io.Closer, error) {
	lockedFile.mu.Lock()
	defer lockedFile.mu.Unlock()

	if lockedFile.files == nil {
		lockedFile.files = make(map[string]bool)
	}

	if _, ok := lockedFile.files[name]; ok {
		return nil, errors.New("lock held by current process")
	}

	osFile, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	spec := unix.Flock_t{
		Type:   unix.F_WRLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0, // 0 means to lock the entire file.
		Pid:    int32(os.Getpid()),
	}
	if err := unix.FcntlFlock(osFile.Fd(), unix.F_SETLK, &spec); err != nil {
		osFile.Close()
		return nil, err
	}
	lockedFile.files[name] = true

	return lockCloser{name, osFile}, nil
}

var _ FS = (*defaultUnix)(nil)
