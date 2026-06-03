# asc-mcp — MCP server for App Store Connect metadata automation.
#
# Common targets:
#   make build      build the server binary
#   make test       run unit tests with the race detector
#   make cover      run tests with coverage
#   make vet        go vet
#   make generate   regenerate the ASC client + mocks
#   make migrate    apply goose migrations to $(DB_DSN)
#   make run        run the server over stdio (needs ASC_* env vars)

GO            ?= go
BIN_DIR       ?= bin
SERVER_BIN    := $(BIN_DIR)/asc-mcp
SPEC_URL      ?= https://raw.githubusercontent.com/RageAgainstThePixel/app-store-connect-api/main/app_store_connect_api_openapi.json
SPEC_FULL     := api/openapi.json
SPEC_PRUNED   := api/openapi.pruned.json
INCLUDE_OPS   := scripts/include_ops.txt
DB_DSN        ?= postgres://postgres:postgres@localhost:5432/asc_mcp?sslmode=disable

OAPI_CODEGEN  := $(GO) tool oapi-codegen
MOCKGEN       := $(GO) tool mockgen
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS       := -s -w -X main.serverVersion=$(VERSION)

.PHONY: all build install test cover vet generate spec prune client mocks migrate run tidy clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) ./cmd/asc-mcp

install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/asc-mcp

test:
	$(GO) test -race ./...

cover:
	$(GO) test -cover ./...

vet:
	$(GO) vet ./...

## Code generation -----------------------------------------------------------

generate: client mocks

# Download the latest Apple OpenAPI spec (community mirror; the official spec
# also lives behind App Store Connect auth at /openapi.json).
spec:
	curl -sSL -o $(SPEC_FULL) $(SPEC_URL)

# Narrow the full spec to the operations the server calls and emit the client.
prune:
	python3 scripts/prune_spec.py $(SPEC_FULL) $(INCLUDE_OPS) $(SPEC_PRUNED)

client: prune
	$(OAPI_CODEGEN) -config api/oapi-codegen.yaml $(SPEC_PRUNED)

mocks:
	$(GO) generate ./...

## Database ------------------------------------------------------------------

migrate:
	$(GO) run github.com/pressly/goose/v3/cmd/goose -dir ./migrations postgres "$(DB_DSN)" up

## Run -----------------------------------------------------------------------

run:
	$(GO) run ./cmd/asc-mcp

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)
