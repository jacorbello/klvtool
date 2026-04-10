.PHONY: fmt lint test test-integration build

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
BUILD_DIR ?= bin
BINARY ?= klvtool

$(GOCACHE):
	mkdir -p $(GOCACHE)

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

fmt:
	$(GO) fmt ./...

lint: $(GOCACHE)
	mkdir -p $(GOLANGCI_LINT_CACHE)
	GOCACHE=$(GOCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(GOLANGCI_LINT) run ./...

test: $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) test ./...

test-integration: $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) test ./integration -v

build: $(GOCACHE) $(BUILD_DIR)
	GOCACHE=$(GOCACHE) $(GO) build -o $(BUILD_DIR)/$(BINARY) ./cmd/klvtool
