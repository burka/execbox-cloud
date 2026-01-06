.PHONY: build build-dashboard prepare-static run dev dev-frontend test test-all lint lint-all check clean deploy deploy-prod logs status generate openapi

# Generate OpenAPI spec from Go code
openapi.json: $(shell find internal -name '*.go') cmd/server/main.go
	go run ./cmd/server --openapi > openapi.json

# Generate TypeScript types from OpenAPI spec
generate: openapi.json
	cd dashboard && npm run generate

# Build dashboard (regenerates types first via openapi.json dependency)
build-dashboard: openapi.json
	cd dashboard && npm run build

# Prepare static files for embedding
prepare-static: build-dashboard
	rm -rf static/dist
	cp -r dashboard/dist static/dist

# Full production build
build: prepare-static
	go build -o bin/execbox-cloud ./cmd/server

# Development build (no embedded dashboard)
build-dev:
	go build -tags dev -o bin/execbox-cloud-dev ./cmd/server

# Run the Go server (backend only)
run:
	go run ./cmd/server

# Run Vite dev server with hot reload (requires backend running separately)
dev-frontend:
	cd dashboard && npm run dev

# Run both backend and frontend in development mode (requires terminal multiplexer or two terminals)
# Usage: Run 'make run' in one terminal, 'make dev-frontend' in another
dev:
	@echo "For development, run these in separate terminals:"
	@echo "  Terminal 1: make run          (Go backend on :8080)"
	@echo "  Terminal 2: make dev-frontend (Vite dev server on :5173)"
	@echo ""
	@echo "Then open http://localhost:5173 in your browser"

# Go tests only
test:
	go test ./...

# Frontend type check and lint
test-frontend: openapi.json
	cd dashboard && npm run generate
	cd dashboard && npm run build
	cd dashboard && npm run lint

# Run all tests (Go + Frontend)
test-all: test test-frontend
	@echo "All tests passed!"

# Go lint only
lint:
	golangci-lint run

# Lint everything (Go + Frontend)
lint-all: lint
	cd dashboard && npm run lint

# Quick check: verify everything compiles without full build
check: openapi.json
	go build ./...
	cd dashboard && npm run generate
	cd dashboard && npm run typecheck
	@echo "All checks passed!"

# Clean build artifacts
clean:
	rm -rf bin/ static/dist dashboard/dist

# Deployment
deploy:
	fly deploy

deploy-prod:
	fly deploy --app execbox-cloud

logs:
	fly logs

status:
	fly status

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Development:"
	@echo "  make dev          - Show instructions for dev mode"
	@echo "  make run          - Run Go backend on :8080"
	@echo "  make dev-frontend - Run Vite dev server on :5173 (hot reload)"
	@echo ""
	@echo "Build:"
	@echo "  make build        - Full production build (Go + embedded dashboard)"
	@echo "  make build-dev    - Development build (no embedded dashboard)"
	@echo "  make generate     - Regenerate OpenAPI spec and TypeScript types"
	@echo "  make clean        - Remove build artifacts"
	@echo ""
	@echo "Test & Lint:"
	@echo "  make test         - Run Go tests"
	@echo "  make test-all     - Run all tests (Go + Frontend + Lint)"
	@echo "  make check        - Quick verification (compile + typecheck)"
	@echo "  make lint-all     - Run all linters (Go + ESLint)"
