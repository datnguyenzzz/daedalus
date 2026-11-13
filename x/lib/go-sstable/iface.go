package go_sstable

// IWriter represent an interface of writer for downstream client to the SSTable
// The data written layout is controlled internally, which caller of this function
// shouldn't worry much about it
type IWriter interface {
	// Set appends key/value pair to the table. It is safe to modify the contents of the arguments after Append returns.
	Set(key, value []byte) error
	// Delete a key within a table
	Delete(key []byte) error
	// Close will finalize the table. Calling Append is not possible after Close
	Close() error
	// TODO(med): support merge operation (read-modify-write loop)
	// TODO(med): support range query (delete, ...)
}
