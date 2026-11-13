package go_fs

import (
	"errors"
	"io"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

type Location byte

const (
	InMemory    Location = iota
	FileSystem           // (Local OS's File system, or remote ones like SeaweedFs, ...)
	BlobStorage          // (eg. S3, GCS, ...)
)

type FileDesc struct {
	Type nogodb_common.ObjectType
	Num  nogodb_common.DiskfileNum
	Loc  Location
}

func FromFileDescToFileNum(fd FileDesc) nogodb_common.DiskfileNum {
	return nogodb_common.DiskfileNum(int64(fd.Num)<<8 | int64(fd.Type))
}

var (
	errFileNotFound = errors.New("file not found")
	errFileIsClosed = errors.New("file is closed")
	errFileExists   = errors.New("file exists")
)

type Syncer interface {
	Sync() error
}

// Writable is the handle for a storage object that is open for writing.
type Writable interface {
	// io.Write writes len(p) bytes from p to the underlying object. The data is not
	// guaranteed to be durable until Finish is called.
	//
	// io.Write make sure that the error will be not nil, if n < len(p)

	io.Writer
	Syncer

	// Finish completes the object and makes the data durable.
	// No further calls are allowed after calling Finish.
	Finish() error

	// Abort gives up on finishing the object. There is no guarantee about whether
	// the object exists after calling Abort.
	// No further calls are allowed after calling Abort.
	Abort()
}

// Readable is the handle for a storage object that is open for reading.
type Readable interface {
	io.ReaderAt
	io.Reader

	Size() uint64
	Close() error
}

// Storage is a singleton object used to access and manage objects.
//
// An object is conceptually like a large immutable file. The main use of
// objects is for storing sstables; in the future it could also be used for blob
// storage.
type Storage interface {
	// Open opens an existing object with the given 'file descriptor' read-only.
	Open(objType nogodb_common.ObjectType, num nogodb_common.DiskfileNum) (Readable, FileDesc, error)

	// Create creates a new object and opens it for writing.
	//
	// The object is not guaranteed to be durable (accessible in case of crashes)
	// until Sync is called.
	Create(objType nogodb_common.ObjectType, num nogodb_common.DiskfileNum) (Writable, FileDesc, error)

	// LookUp returns the metadata of an object that is already exists
	// it doesn't perform any I/O operations
	LookUp(objType nogodb_common.ObjectType, num nogodb_common.DiskfileNum) (FileDesc, error)

	Remove(objType nogodb_common.ObjectType, num nogodb_common.DiskfileNum) error

	Sync(objType nogodb_common.ObjectType, num nogodb_common.DiskfileNum) error

	List(objType nogodb_common.ObjectType) []FileDesc

	Close() error
}
