module github.com/datnguyenzzz/nogodb/lib/go-adaptive-radix-tree

replace github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock => ../go-context-aware-lock

go 1.26.0

require (
	github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock v0.0.0-20260207164643-7fe89bf3da87
	github.com/go-faker/faker/v4 v4.6.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.19.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
