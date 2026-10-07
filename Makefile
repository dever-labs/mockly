BINARY     := mockly
UI_DIR     := ui
ASSETS_DIR := assets
DIST_DIR   := dist

GO_BUILD_FLAGS := -ldflags="-s -w"

.PHONY: all build build-ui build-go clean test test-e2e lint dev tidy test-tc test-tc-go test-tc-node test-tc-python

all: build

## build: Build UI then embed into Go binary
build: build-ui build-go

## build-ui: Build the React UI
# vite.config.ts sets build.outDir to "../assets/dist", so the UI build
# writes straight into $(ASSETS_DIR)/$(DIST_DIR) — no separate copy step
# needed (and none is portable across Windows/Linux/macOS without extra
# tooling, so don't reintroduce one).
build-ui:
	@echo "→ Building UI..."
	cd $(UI_DIR) && npm ci && npm run build

## build-go: Compile the Go binary
build-go:
	@echo "→ Building Go binary..."
	go build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/mockly

## clean: Remove build artefacts
clean:
	rm -rf $(ASSETS_DIR)/$(DIST_DIR)
	rm -f $(BINARY) $(BINARY).exe
	cd $(UI_DIR) && rm -rf dist

## test: Run unit and integration tests
test:
	go test ./internal/... -v -race -coverprofile=coverage.txt

## test-e2e: Run end-to-end tests (builds the UI + binary first)
test-e2e: build
	go test -tags e2e ./tests/e2e/... -v -timeout 120s

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## dev: Run with hot-reload (requires air)
dev:
	air

## tidy: Tidy Go modules
tidy:
	go mod tidy

## test-tc: Run Testcontainers integration tests for all supported languages (requires Docker)
test-tc: test-tc-go test-tc-node test-tc-python

## test-tc-go: Run Go Testcontainers integration tests
test-tc-go:
	cd clients/go/testcontainers && go test -tags integration -timeout 120s -v ./...

## test-tc-node: Run Node.js Testcontainers integration tests
test-tc-node:
	cd clients/node-testcontainers && npm run test:integration

## test-tc-python: Run Python Testcontainers integration tests
test-tc-python:
	cd clients/python-testcontainers && python3 -m pytest -m integration tests/ -v
