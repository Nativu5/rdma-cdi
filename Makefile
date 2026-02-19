MODULE   := github.com/Nativu5/rdma-cdi
BINARY   := rdma-cdi
CMD_DIR  := ./cmd/rdma-cdi
PREFIX   ?= /usr/local/bin
VERBOSE  ?=

# Build flags
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS  := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)
GOFLAGS  ?=

TEST_FLAGS := -count=1
ifneq ($(VERBOSE),)
TEST_FLAGS += -v
endif

.PHONY: all build test coverage check clean install uninstall help

all: check build ## Run all checks and build (default)

## ── Build ──────────────────────────────────────

build: ## Build the binary
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) $(CMD_DIR)

install: build ## Install the binary to PREFIX (default /usr/local/bin)
	install -d $(PREFIX)
	install -m 0755 $(BINARY) $(PREFIX)/$(BINARY)

uninstall: ## Remove the installed binary
	rm -f $(PREFIX)/$(BINARY)

clean: ## Remove build artifacts
	rm -f $(BINARY)
	go clean -testcache

## ── Test ───────────────────────────────────────

test: ## Run unit tests (VERBOSE=1 for verbose output)
	go test ./... $(TEST_FLAGS)

coverage: ## Run tests with coverage report
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo "── HTML report: coverage.html ──"
	go tool cover -html=coverage.out -o coverage.html

## ── Code Quality ───────────────────────────────

check: ## Run gofmt, go vet, and staticcheck
	@echo "── gofmt ──"
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need formatting:"; \
		echo "$$unformatted"; \
		gofmt -w .; \
		echo "Fixed."; \
	fi
	@echo "── go vet ──"
	go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		echo "── staticcheck ──"; \
		staticcheck ./...; \
	fi

## ── Help ───────────────────────────────────────

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
