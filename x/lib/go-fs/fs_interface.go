package go_fs

import (
	"io"
	"os"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// File System interfaces

// File is a readable, writable sequence of bytes.
// Write-oriented operations (Write, Sync) must be called sequentially: At most
// 1 call to Write or Sync may be executed at any given time.
type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Writer
	// WriteAt() is only supported for files that were opened with FS.OpenReadWrite.
	io.WriterAt

	Syncer
}

type FileInfo interface {
	os.FileInfo
}

// DiskUsage summarizes disk space usage on a filesystem.
type DiskUsage struct {
	// Total disk space available to the current process in bytes.
	AvailBytes uint64
	// Total disk space in bytes.
	TotalBytes uint64
	// Used disk space in bytes.
	UsedBytes uint64
}

type FS interface {
	// Create creates the named file for reading and writing. If a file
	// already exists at the provided name, it's removed first ensuring the
	// resulting file descriptor points to a new inode.
	Create(name string, objType nogodb_common.ObjectType) (File, error)

	// Open opens the named file for reading. openOptions provides
	Open(name string) (File, error)

	// // OpenReadWrite opens the named file for reading and writing. If the file
	// // does not exist, it is created.
	// OpenReadWrite(name string, category ObjectType) (File, error)

	// OpenDir opens the named directory for syncing.
	OpenDir(name string) (File, error)

	// Remove removes the named file or directory.
	Remove(name string) error

	// Lock locks the given file, creating the file if necessary, and
	// truncating the file if it already exists. The lock is an exclusive lock
	// (a write lock), but locked files should neither be read from nor written
	// to. Such files should have zero size and only exist to co-ordinate
	// ownership across processes.
	//
	// Attempting to lock a file that is already locked by the current process
	// returns an error and leaves the existing lock untouched.
	Lock(name string) (io.Closer, error)

	// // Remove removes the named file or directory and any children it
	// // contains. It removes everything it can but returns the first error it
	// // encounters.
	// RemoveAll(name string) error

	// MkdirAll creates a directory and all necessary parents. The permission
	// bits perm have the same semantics as in os.MkdirAll. If the directory
	// already exists, MkdirAll does nothing and returns nil.
	MkdirAll(dir string, perm os.FileMode) error

	// List returns a listing of the given directory. The names returned are
	// relative to dir.
	List(dir string) ([]string, error)

	// Stat returns an FileInfo describing the named file.
	Stat(name string) (FileInfo, error)

	// PathBase returns the last element of path. Trailing path separators are
	// removed before extracting the last element. If the path is empty, PathBase
	// returns ".".  If the path consists entirely of separators, PathBase returns a
	// single separator.
	PathBase(path string) string

	// PathJoin joins any number of path elements into a single path, adding a
	// separator if necessary.
	PathJoin(elem ...string) string

	// PathDir returns all but the last element of path, typically the path's directory.
	PathDir(path string) string

	// // GetDiskUsage returns disk space statistics for the filesystem where
	// // path is any file or directory within that filesystem.
	// GetDiskUsage(path string) (DiskUsage, error)
}
