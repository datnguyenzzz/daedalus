module github.com/datnguyenzzz/nogodb/lib/go-wal

go 1.26.0

replace github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool => ../go-bytesbufferpool

replace github.com/datnguyenzzz/nogodb/lib/go-fs => ../go-fs

replace github.com/datnguyenzzz/nogodb/lib/common => ../common

require (
	github.com/datnguyenzzz/nogodb/lib/common v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-fs v0.0.0-00010101000000-000000000000
)

require (
	github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/sys v0.44.0 // indirect
)
