module github.com/datnguyenzzz/nogodb/lib/go-sstable

go 1.26.0

replace github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter => ../go-blocked-bloom-filter

replace github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool => ../go-bytesbufferpool

replace github.com/datnguyenzzz/nogodb/lib/go-fs => ../go-fs

replace github.com/datnguyenzzz/nogodb/lib/go-block-cache => ../go-block-cache

replace github.com/datnguyenzzz/nogodb/lib/common => ../common

require (
	github.com/datnguyenzzz/nogodb/lib/common v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-block-cache v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool v0.0.0-20250609152930-352a93d7ed86
	github.com/datnguyenzzz/nogodb/lib/go-fs v0.0.0-00010101000000-000000000000
	github.com/go-faker/faker/v4 v4.7.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.19.0
)

require (
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
