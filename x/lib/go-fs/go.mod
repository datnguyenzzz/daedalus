module github.com/datnguyenzzz/nogodb/lib/go-fs

go 1.26.0

replace github.com/datnguyenzzz/nogodb/lib/common => ../common

replace github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool => ../go-bytesbufferpool

require (
	github.com/datnguyenzzz/nogodb/lib/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.44.0
)

require (
	github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
