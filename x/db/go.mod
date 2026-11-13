module github.com/datnguyenzzz/nogodb/db

go 1.26.0

replace github.com/datnguyenzzz/nogodb/lib/go-block-cache => ../lib/go-block-cache

replace github.com/datnguyenzzz/nogodb/lib/go-fs => ../lib/go-fs

replace github.com/datnguyenzzz/nogodb/lib/go-wal => ../lib/go-wal

replace github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool => ../lib/go-bytesbufferpool

replace github.com/datnguyenzzz/nogodb/lib/common => ../lib/common

replace github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree => ../lib/go-adaptive-radix-tree

replace github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock => ../lib/go-context-aware-lock

replace github.com/datnguyenzzz/nogodb/lib/go-sstable => ../lib/go-sstable

replace github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter => ../lib/go-blocked-bloom-filter

require (
	github.com/datnguyenzzz/nogodb/lib/common v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-block-cache v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-bytesbufferpool v0.0.0-20250609152930-352a93d7ed86
	github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock v0.0.0-20260207164643-7fe89bf3da87
	github.com/datnguyenzzz/nogodb/lib/go-fs v0.0.0-00010101000000-000000000000
	github.com/datnguyenzzz/nogodb/lib/go-wal v0.0.0-00010101000000-000000000000
	github.com/google/btree v1.1.3
	golang.org/x/sync v0.19.0
)

require github.com/datnguyenzzz/nogodb/lib/go-sstable v0.0.0-00010101000000-000000000000

require (
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/datnguyenzzz/nogodb/lib/go-blocked-bloom-filter v0.0.0-00010101000000-000000000000 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-faker/faker/v4 v4.7.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
