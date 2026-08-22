# TechPulse Makefile
# Provides common development commands

.PHONY: build build-release test test-cover test-cover-report test-cover-html test-module fmt vet lint run run-daemon clean help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
BINARY_NAME=techpulse
COVER_FILE=coverage.out

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X github.com/majiayu000/techpulse/internal/techpulse.Version=$(VERSION) \
	-X github.com/majiayu000/techpulse/internal/techpulse.BuildDate=$(BUILD_DATE) \
	-X github.com/majiayu000/techpulse/internal/techpulse.GitCommit=$(GIT_COMMIT)"

# Build the binary (simple, for development)
build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/techpulse

# Build with version info embedded
build-release:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/techpulse

# Run all tests
test:
	$(GOTEST) -v ./internal/...

# Run tests with coverage summary
test-cover:
	@echo "Running tests with coverage..."
	@$(GOTEST) -cover ./internal/... | tee /dev/stderr | \
		awk '/^ok/ {total += $$5; count++} END {if(count>0) printf "\n=== Average Coverage: %.1f%% ===\n", total/count}'

# Generate detailed coverage report
test-cover-report:
	@echo "Generating coverage report..."
	$(GOTEST) -coverprofile=$(COVER_FILE) ./internal/...
	@echo "\n=== Coverage Summary ==="
	@$(GOCMD) tool cover -func=$(COVER_FILE) | tail -1
	@echo ""
	@echo "Coverage file saved to $(COVER_FILE)"
	@echo "Run 'make test-cover-html' to view in browser"

# Generate HTML coverage report
test-cover-html: test-cover-report
	$(GOCMD) tool cover -html=$(COVER_FILE) -o coverage.html
	@echo "HTML report saved to coverage.html"
	@echo "Opening in browser..."
	@open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "Open coverage.html manually"

# Run a single test module with coverage
test-module:
	@if [ -z "$(MODULE)" ]; then echo "Usage: make test-module MODULE=collector/hackernews"; exit 1; fi
	$(GOTEST) -v -cover ./internal/$(MODULE)/...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(COVER_FILE)
	rm -f coverage.html

# Run lint checks (vet + gofmt gate; fails loudly instead of silent pass)
lint:
	$(GOCMD) vet ./...
	@files="$$(gofmt -l .)"; if [ -n "$$files" ]; then echo "gofmt needed for:"; echo "$$files"; exit 1; fi
	@echo "lint OK"

# Run go fmt
fmt:
	$(GOCMD) fmt ./...

# Run go vet
vet:
	$(GOCMD) vet ./...

# Run the tool
run:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/techpulse && ./$(BINARY_NAME)

# Run in daemon mode
run-daemon:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/techpulse && ./$(BINARY_NAME) --daemon --interval 1h

# Show help
help:
	@echo "TechPulse Development Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build             Build the techpulse binary (dev)"
	@echo "  build-release     Build with version info embedded"
	@echo "  test              Run all tests"
	@echo "  test-cover        Run tests with coverage summary"
	@echo "  test-cover-report Generate coverage profile (coverage.out)"
	@echo "  test-cover-html   Generate HTML coverage report"
	@echo "  test-module       Test specific module (MODULE=collector/hackernews)"
	@echo "  fmt               Format code"
	@echo "  vet               Run go vet"
	@echo "  lint              Run vet + gofmt gate"
	@echo "  run               Build and run"
	@echo "  run-daemon        Build and run in daemon mode"
	@echo "  clean             Remove build artifacts"
	@echo "  help              Show this help"
