.PHONY: fmt lint test test-integration test-data build man man-check

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
BUILD_DIR ?= bin
BINARY ?= klvtool
VERSION ?= dev
SAMPLE_TS ?= testdata/fixtures/sample.ts
SAMPLE_SHA256 ?= 8667276b2c2fb36baa089b00e3978f55893cacf6e0d8f6e6d480bb934747cc79
SAMPLE_URL ?= https://www.arcgis.com/sharing/rest/content/items/55ec6f32d5e342fcbfba376ca2cc409a/data
SAMPLE_ZIP ?= $(CURDIR)/.cache/FMV_tutorial_data.zip
MAN_DIR ?= man/man1
# CI uses a fixed date for deterministic drift detection; local `make man`
# stamps today's date so released pages show real timestamps.
MAN_DATE ?= $(shell date -u +%Y-%m-%d)

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

test-data:
	@if [ -f $(SAMPLE_TS) ] && echo "$(SAMPLE_SHA256)  $(SAMPLE_TS)" | sha256sum -c --quiet 2>/dev/null; then \
		echo "testdata/fixtures/sample.ts already present and verified."; \
	else \
		echo "Downloading FMV tutorial data..."; \
		mkdir -p .cache; \
		curl -L -o $(SAMPLE_ZIP) $(SAMPLE_URL); \
		echo "Extracting sample.ts..."; \
		mkdir -p testdata/fixtures; \
		unzip -o -j $(SAMPLE_ZIP) "FMV tutorial data/Truck.ts" -d testdata/fixtures; \
		mv testdata/fixtures/Truck.ts $(SAMPLE_TS); \
		echo "Verifying SHA256..."; \
		echo "$(SAMPLE_SHA256)  $(SAMPLE_TS)" | sha256sum -c --quiet; \
		echo "Cleaning up..."; \
		rm -f $(SAMPLE_ZIP); \
		echo "Done. testdata/fixtures/sample.ts is ready."; \
	fi

build: $(GOCACHE) $(BUILD_DIR)
	GOCACHE=$(GOCACHE) $(GO) build -ldflags "-X github.com/jacorbello/klvtool/internal/version.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY) ./cmd/klvtool

# Regenerate man pages from internal/cli CommandDefs. Local builds get a real
# date; CI overrides with MAN_DATE=1970-01-01 via man-check.
man: $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) run ./cmd/gen-manpages -out $(MAN_DIR) -date $(MAN_DATE) -version $(VERSION)

# Drift check used in CI: regenerate with a fixed date AND a fixed version so
# the diff is deterministic regardless of how a developer invoked `make man`
# locally (e.g. with VERSION=1.2.0). Committed pages are always the
# placeholder values; GoReleaser stamps the real version + date at release
# time. Run `make man` locally and commit the result to fix drift.
man-check: $(GOCACHE)
	GOCACHE=$(GOCACHE) $(GO) run ./cmd/gen-manpages -out $(MAN_DIR) -date 1970-01-01 -version dev
	@git diff --exit-code -- $(MAN_DIR) || (echo "ERROR: man pages are out of date. Run 'make man' and commit the result." && exit 1)
