.PHONY: build clean vet

BINARY_NAME := d3-movie-downloader
BINARY_DIR := bin
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)

# Go build flags
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 1

build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY_PATH) ./cmd/bot
	@echo "✅ Binary built: $(BINARY_PATH)"

clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BINARY_DIR)
	@echo "✅ Clean done"

vet:
	@echo "🔍 Running go vet..."
	go vet ./...
	@echo "✅ Vet done"
