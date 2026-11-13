unit-test:
	@go test -count=1 -race -v ./...

functional-test:
	@go test -tags=functional_tests -count=1 -p 1 -v ./...

benchmark:
	@go test -benchmem -bench=. | tee bench-results.txt