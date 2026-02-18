.PHONY: all test lint build clean generate help

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## [a-zA-Z_-]+: .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ": "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' | sed 's/## //'

## test: Run all tests with race detector and coverage
test:
	go test -v -race -cover ./...

## lint: Run golangci-lint
lint:
	golangci-lint run

## build: Build the library and the generator tool
build:
	go build ./...
	mkdir -p bin
	go build -o bin/confusables-gen ./cmd/confusables-gen/main.go

## generate: Regenerate the embedded JSON data (requires network)
generate:
	go run ./cmd/confusables-gen/main.go --output data/confusables.json

## clean: Remove build artifacts and coverage files
clean:
	rm -rf bin/
	rm -f coverage.out
