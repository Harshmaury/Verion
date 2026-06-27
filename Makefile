SHELL := /bin/bash
PATH := /usr/local/go/bin:$(PATH)

.PHONY: help build test lint clean run dev

BINARY     = verion
CMD_PATH   = ./cmd/verion
BUILD_DIR  = bin

help:           ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build:          ## Build the binary
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY)"

test:           ## Run all tests
	go test ./... -v -race

lint:           ## Run linter (requires golangci-lint)
	golangci-lint run ./...

clean:          ## Remove build artifacts
	rm -rf $(BUILD_DIR)

run:            ## Build and run
	go run $(CMD_PATH)/main.go

tidy:           ## Tidy go.mod
	go mod tidy

fmt:            ## Format code
	gofmt -w .
