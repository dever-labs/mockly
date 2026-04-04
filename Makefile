BINARY     := mockly
UI_DIR     := ui
ASSETS_DIR := assets
DIST_DIR   := dist

GO_BUILD_FLAGS := -ldflags="-s -w"

.PHONY: all build build-ui build-go clean test lint dev

all: build

## build: Build UI then embed into Go binary
build: build-ui build-go

## build-ui: Build the React UI
build-ui:
	@echo "→ Building UI..."
	cd $(UI_DIR) && npm ci && npm run build
	@echo "→ Copying UI dist to assets..."
	if not exist $(ASSETS_DIR) mkdir $(ASSETS_DIR)
	xcopy /E /Y /I $(UI_DIR)\dist $(ASSETS_DIR)\dist

## build-go: Compile the Go binary
build-go:
	@echo "→ Building Go binary..."
	go build $(GO_BUILD_FLAGS) -o $(BINARY).exe ./cmd/mockly

## clean: Remove build artefacts
clean:
	rm -rf $(ASSETS_DIR)\dist
	rm -f $(BINARY) $(BINARY).exe
	cd $(UI_DIR) && rm -rf dist

## test: Run unit and integration tests
test:
	go test ./internal/... -v -race -coverprofile=coverage.txt

## test-e2e: Run end-to-end tests (builds binary first)
test-e2e: build-go
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
