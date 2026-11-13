module github.com/datnguyenzzz/nogodb/lib/common

go 1.26.0

replace github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool => ../go-bytesbufferpool

require (
	github.com/DataDog/zstd v1.5.7
	github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool v0.0.0-00010101000000-000000000000
	github.com/golang/snappy v1.0.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
