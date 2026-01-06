.PHONY: build build-dashboard prepare-static run dev dev-frontend test test-all lint lint-all check clean deploy deploy-prod logs status generate openapi run-devdb stop-devdb setup-env dev-with-db dev-setup

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

# Environment Setup
setup-env:
	@if [ ! -f .env ]; then \
		echo "Creating .env file with defaults..."; \
		echo "# Database Configuration" > .env; \
		echo "DATABASE_URL=postgresql://postgres:postgres@localhost:5433/execbox" >> .env; \
		echo "DB_HOST=localhost" >> .env; \
		echo "DB_PORT=5433" >> .env; \
		echo "DB_USER=postgres" >> .env; \
		echo "DB_PASSWORD=postgres" >> .env; \
		echo "DB_NAME=execbox" >> .env; \
		echo "" >> .env; \
		echo "# Fly.io API (required for machine management)" >> .env; \
		echo "FLY_API_TOKEN=your-fly-api-token-here" >> .env; \
		echo "" >> .env; \
		echo "# Application Settings" >> .env; \
		echo "LOG_LEVEL=debug" >> .env; \
		echo "PORT=8080" >> .env; \
		echo "" >> .env; \
		echo "# Frontend Development" >> .env; \
		echo "VITE_API_BASE_URL=http://localhost:8080" >> .env; \
		echo "Environment file created. Please update FLY_API_TOKEN and other values as needed."; \
	else \
		echo "Environment file already exists"; \
	fi

# Development Database with Docker Compose
run-devdb: setup-env
	@echo "🚀 Starting development database..."
	@if ! docker --version > /dev/null 2>&1; then \
		echo "❌ Docker is required but not installed. Please install Docker first."; \
		exit 1; \
	fi
	@if ! docker-compose --version > /dev/null 2>&1; then \
		echo "❌ Docker Compose is required but not installed. Please install Docker Compose first."; \
		exit 1; \
	fi
	@if [ ! -f docker-compose.yml ]; then \
		echo "Creating docker-compose.yml..."; \
		echo "version: '3.8'" > docker-compose.yml; \
		echo "" >> docker-compose.yml; \
		echo "services:" >> docker-compose.yml; \
		echo "  postgres:" >> docker-compose.yml; \
		echo "    image: postgres:15-alpine" >> docker-compose.yml; \
		echo "    container_name: execbox-postgres" >> docker-compose.yml; \
		echo "    environment:" >> docker-compose.yml; \
		echo "      POSTGRES_DB: execbox" >> docker-compose.yml; \
		echo "      POSTGRES_USER: postgres" >> docker-compose.yml; \
		echo "      POSTGRES_PASSWORD: postgres" >> docker-compose.yml; \
		echo "    ports:" >> docker-compose.yml; \
		echo "      - 5433:5432" >> docker-compose.yml; \
		echo "    volumes:" >> docker-compose.yml; \
		echo "      - postgres_data:/var/lib/postgresql/data" >> docker-compose.yml; \
		echo "    healthcheck:" >> docker-compose.yml; \
		echo "      test: ['CMD-SHELL', 'pg_isready -U postgres -d execbox']" >> docker-compose.yml; \
		echo "      interval: 5s" >> docker-compose.yml; \
		echo "      timeout: 5s" >> docker-compose.yml; \
		echo "      retries: 5" >> docker-compose.yml; \
		echo "    restart: unless-stopped" >> docker-compose.yml; \
		echo "" >> docker-compose.yml; \
		echo "volumes:" >> docker-compose.yml; \
		echo "  postgres_data:" >> docker-compose.yml; \
		echo "    driver: local" >> docker-compose.yml; \
	fi
	docker-compose up -d postgres
	@echo "⏳ Waiting for database to be ready..."
	@sleep 5
	@until docker-compose exec -T postgres pg_isready -U postgres -d execbox > /dev/null 2>&1; do \
		echo "⏳ Waiting for postgres..."; \
		sleep 2; \
	done
	@echo "✅ Database is ready at postgresql://postgres:postgres@localhost:5433/execbox"
	@echo ""
	@echo "📝 Next steps:"
	@echo "   make run       # Start the backend server"
	@echo "   make dev-db    # Start backend + frontend with database"

# Stop Development Database
stop-devdb:
	@echo "🛑 Stopping development database..."
	docker-compose down
	@echo "✅ Database stopped"

# Reset Development Database (delete all data)
reset-devdb: stop-devdb
	@echo "🗑️  Removing database volume and data..."
	docker volume rm execbox-cloud_postgres_data 2>/dev/null || true
	$(MAKE) run-devdb
	@echo "✅ Database reset with fresh data"

# Development with Database (Full Stack)
dev-db: run-devdb
	@echo ""
	@echo "🚀 Starting full development stack..."
	@echo ""
	@echo "📝 Development environment is ready!"
	@echo ""
	@echo "🔗 Services:"
	@echo "   • Database: postgresql://postgres:postgres@localhost:5433/execbox"
	@echo "   • Backend:  http://localhost:8080 (API + OpenAPI)"
	@echo "   • Frontend: http://localhost:5173 (React Dashboard)"
	@echo ""
	@echo "🎯 Quick start:"
	@echo "   Terminal 1: make run           # Start backend server"
	@echo "   Terminal 2: make dev-frontend  # Start frontend dev server"
	@echo ""
	@echo "🔍 Useful commands:"
	@echo "   • Database:    docker-compose logs postgres"
	@echo "   • Backend:     Logs will appear in terminal"
	@echo "   • Frontend:    Hot reload enabled in browser"
	@echo "   • Stop all:    make stop-devdb"

# Development Mode with Auto-setup
dev-with-db: setup-env run-devdb
	@echo ""
	@echo "🎯 Quick development setup complete!"
	@echo "Run these in separate terminals:"
	@echo ""
	@echo "   make run          # Backend (port 8080)"
	@echo "   make dev-frontend # Frontend (port 5173)"
	@echo ""
	@echo "Then visit: http://localhost:5173"

# Complete Development Setup (automated script)
dev-setup:
	@echo "🚀 Running complete development setup..."
	@echo "This will install dependencies, setup environment, and start database."
	@echo ""
	./scripts/dev-setup.sh

# Database Commands
db-migrate:
	@echo "🔄 Running database migrations..."
	source .env && go run ./cmd/server --migrate-only
	@echo "✅ Migrations completed"

db-shell:
	@echo "🐘 Opening database shell..."
	docker-compose exec postgres psql -U postgres -d execbox

db-logs:
	docker-compose logs -f postgres

db-status:
	@echo "📊 Database status:"
	@docker-compose ps postgres 2>/dev/null || echo "Database not running. Use 'make run-devdb' to start it."

# Environment Check
check-env:
	@echo "🔍 Checking environment configuration..."
	@if [ ! -f .env ]; then \
		echo "❌ .env file not found. Run 'make setup-env' to create it."; \
		exit 1; \
	fi
	@echo "✅ .env file exists"
	@if grep -q "your-fly-api-token-here" .env; then \
		echo "⚠️  WARNING: FLY_API_TOKEN is still set to placeholder value"; \
	fi
	@if docker-compose ps postgres 2>/dev/null | grep -q "Up"; then \
		echo "✅ Database container running"; \
	else \
		echo "⚠️  Database container not running. Use 'make run-devdb' to start it."; \
	fi

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
	@echo "🚀 Quick Start:"
	@echo "  make dev-setup    - Complete automated dev setup (recommended)"
	@echo "  make dev-with-db  - Complete dev setup (env + db + instructions)"
	@echo "  make dev-db       - Start database + show setup instructions"
	@echo ""
	@echo "🔧 Environment & Database:"
	@echo "  make setup-env    - Create .env file with defaults"
	@echo "  make run-devdb    - Start PostgreSQL via Docker Compose"
	@echo "  make stop-devdb   - Stop development database"
	@echo "  make reset-devdb  - Reset database (delete all data)"
	@echo "  make check-env    - Verify environment configuration"
	@echo ""
	@echo "💾 Database Commands:"
	@echo "  make db-migrate   - Run database migrations"
	@echo "  make db-shell     - Open PostgreSQL shell"
	@echo "  make db-logs      - Show database logs"
	@echo "  make db-status    - Show database status"
	@echo ""
	@echo "🏃 Development:"
	@echo "  make run          - Run Go backend on :8080"
	@echo "  make dev-frontend - Run Vite dev server on :5173 (hot reload)"
	@echo "  make dev          - Show development mode instructions"
	@echo ""
	@echo "🏗️  Build:"
	@echo "  make build        - Full production build (Go + embedded dashboard)"
	@echo "  make build-dev    - Development build (no embedded dashboard)"
	@echo "  make generate     - Regenerate OpenAPI spec and TypeScript types"
	@echo "  make clean        - Remove build artifacts"
	@echo ""
	@echo "🧪 Test & Lint:"
	@echo "  make test         - Run Go tests"
	@echo "  make test-all     - Run all tests (Go + Frontend + Lint)"
	@echo "  make check        - Quick verification (compile + typecheck)"
	@echo "  make lint-all     - Run all linters (Go + ESLint)"
	@echo ""
	@echo "🌐 Deployment:"
	@echo "  make deploy       - Deploy to Fly.io"
	@echo "  make deploy-prod  - Deploy to production"
	@echo "  make status       - Check Fly.io app status"
	@echo "  make logs         - Show Fly.io logs"
