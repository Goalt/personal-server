# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary name
BINARY_NAME=personal-server
BINARY_UNIX=$(BINARY_NAME)_unix

# Main package path
MAIN_PACKAGE_PATH=./cmd

# Build directory
BUILD_DIR=./bin

# Version info
# Get version from git tags, fallback to "dev" if no tags exist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/Goalt/personal-server/internal/app.Version=$(VERSION)"

.PHONY: all build clean test coverage deps fmt vet run help e2e-test

# Default target
all: clean deps test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PACKAGE_PATH)

# Build for Linux
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_UNIX) -v $(MAIN_PACKAGE_PATH)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run e2e tests (requires KinD cluster)
e2e-test: build
	@echo "Running e2e tests..."
	@echo "Note: This requires a running Kubernetes cluster (e.g., KinD)"
	cd test/e2e && $(GOTEST) -v -timeout 10m -run TestNamespaceE2E

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GOCMD) run $(MAIN_PACKAGE_PATH)

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(MAIN_PACKAGE_PATH)

# Development workflow: format, vet, test, build
dev: fmt vet test build

# Show help
help:
	@echo "Available targets:"
	@echo "  all         - Clean, download deps, test, and build"
	@echo "  build       - Build the binary"
	@echo "  build-linux - Build for Linux (cross-compile)"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  e2e-test    - Run e2e tests (requires Kubernetes cluster)"
	@echo "  coverage    - Run tests with coverage report"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  run         - Run the application"
	@echo "  install     - Build and install the binary"
	@echo "  dev         - Development workflow (fmt, vet, test, build)"
	@echo "  help        - Show this help message"