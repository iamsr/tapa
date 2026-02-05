.PHONY: build test lint clean install fix-macos

VERSION := 0.1.0
BUILD_DIR := bin
BINARY_NAME := dma

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/dma

test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.txt

install: build
	@echo "Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

fix-macos:
	@echo "Fixing pg_query_go for macOS..."
	@./scripts/fix_pg_query_macos.sh

.DEFAULT_GOAL := build
