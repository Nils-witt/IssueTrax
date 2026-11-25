BINARY_NAME := issuetrax
MODULE      := nilswitt.dev/issuetrax
GO          ?= go
NPM         ?= npm

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -ldflags "-s -w -X $(MODULE)/internal/version.Version=$(VERSION) -X $(MODULE)/internal/version.Commit=$(COMMIT)"

FRONTEND_DIR := frontend

.PHONY: all build frontend frontend-deps run dev-backend dev-frontend dev-db generate test test-unit lint lint-fix fmt audit clean help

all: lint test build

## build: Build the frontend, then the Go binary with the frontend embedded
build: frontend
	$(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

## frontend-deps: Install frontend dependencies
frontend-deps:
	cd $(FRONTEND_DIR) && $(NPM) ci

## frontend: Build the Vite app into frontend/dist (embedded by go:embed)
frontend:
	cd $(FRONTEND_DIR) && $(NPM) run build

## run: Build and run the server
run: build
	./bin/$(BINARY_NAME)

## dev-db: Start the local development services (PostgreSQL, Authentik on :9000) in Docker
dev-db:
	docker compose -f deploy/compose.yaml up -d

## dev-backend: Run the Go server with the settings from .env (needs `make dev-db`; open the app via `make dev-frontend`)
dev-backend: .env
	set -a && . ./.env && set +a && $(GO) run ./cmd/$(BINARY_NAME)

# Local dev settings (git-ignored), created from the committed template.
.env:
	cp .env.example $@

## generate: Regenerate sqlc database code from internal/adapter/storage/postgres/queries
generate:
	$(GO) generate ./...

## dev-frontend: Run Vite with HMR, proxying /api to the Go server
dev-frontend:
	cd $(FRONTEND_DIR) && $(NPM) run dev

## test: Run all Go tests incl. Postgres integration tests (Docker) with the race detector
test: frontend
	$(GO) test -race -tags integration -coverprofile=coverage.out ./...

## test-unit: Run Go unit tests only (no Docker needed)
test-unit: frontend
	$(GO) test -race ./...

## lint: Run golangci-lint and oxlint
lint: frontend
	golangci-lint run ./...
	cd $(FRONTEND_DIR) && $(NPM) run lint

## lint-fix: Auto-fix lint issues
lint-fix: frontend
	golangci-lint run --fix ./...
	cd $(FRONTEND_DIR) && $(NPM) run lint -- --fix

## fmt: Format Go code
fmt:
	golangci-lint fmt ./...

## audit: Scan Go dependencies for known vulnerabilities
audit:
	govulncheck ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out $(FRONTEND_DIR)/dist

## help: Show this help
help:
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
