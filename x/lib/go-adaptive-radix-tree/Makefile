unit-test: ## test whole packages with the race detector
	@go test -coverprofile=coverage.out -count=1 -race -v ./...

benchmark:
	@go test -benchmem -benchtime=1x -timeout=60m -bench=.

pre-push:
	@echo "✔️ Running go mod tidy and go mod verify..."
	@go mod tidy -v
	@go mod verify
	@echo "✔️ Running gofmt..."
	@gofmt -l .
	@echo "✔️ Running go vet..."
	@go vet ./...