package db

import (
	"io"
)

func (d *DB) Close() error {
	panic("unimplemented")
}

func (d *DB) Get(key []byte) (value []byte, closer io.Closer, err error) {
	panic("unimplemented")
}
