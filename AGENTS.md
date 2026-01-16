# Agent Guidelines for execbox-cloud

Instructions for AI agents working on this codebase.

## Core Principles

1. **Single Responsibility** - Each agent owns their domain completely
2. **Clear Interfaces** - Agents coordinate through defined boundaries
3. **Fail Fast** - Return errors immediately, don't hide failures
4. **Test Completeness** - All database migrations, API handlers, and streams must be tested

## Architecture Overview

```
Client → API (chi/huma) → Auth Middleware → Handler Logic
                                              ↓
                                        Session Manager
                                              ↓
                          Fly.io Machines API + Database
```

**Key layers:**
- HTTP API: REST endpoints for session lifecycle
- WebSocket: Binary protocol for I/O attachment and streaming
- Backend: Fly.io Machines API integration
- Database: Supabase (schema, sessions, metrics, API keys)

## Quick Commands

```bash
# Build
go build -o bin/execbox-cloud ./cmd/server

# Run locally (requires Fly API key + Supabase credentials)
FLY_API_TOKEN=... DATABASE_URL=... go run ./cmd/server

# Test database migrations
go test ./internal/db/...

# Test API handlers
go test ./internal/api/...

# Test WebSocket protocol
go test ./internal/api/websocket_test.go

# Run all tests with full backend (recommended)
./scripts/run-integration-tests.sh

# Run integration tests manually (requires K8s backend running)
go test -tags integration ./test/integration/... -v -timeout 5m
```

## Database Agent

**Expertise:** Supabase, PostgreSQL, pgx, SQL migrations

**Owns:**
- `internal/db/migrations/*.sql` - Database schema
- `internal/db/client.go` - Connection pooling and client lifecycle
- `internal/db/models.go` - Query methods and data types
- `internal/db/*.go` - All database logic

**Responsibilities:**
- Design schema: tables (api_keys, sessions, usage_metrics, machines)
- Implement migrations with rollback safety
- Create query methods for all data access patterns
- Connection pooling configuration and health checks
- Transaction management for atomic operations
- Index design for query performance

**Key Tasks:**
- Add new table for image cache in Phase 2
- Implement retention policies for sessions and metrics
- Write efficient queries for usage aggregation

**Coordination Notes:**
- Consult with Backend Agent before changing session table schema
- Consult with Auth Agent before modifying api_keys table structure
- All schema changes must include migration files with version numbers
- Do not modify committed migrations (create new ones instead)

---

## Backend Agent

**Expertise:** Fly.io Machines API, container lifecycle, lifecycle events

**Owns:**
- `internal/backend/fly/machines.go` - Machine CRUD operations
- `internal/backend/fly/client.go` - Fly API client wrapper
- `internal/backend/fly/attach.go` - Machine attach and detach
- `internal/backend/fly/cleanup.go` - Graceful shutdown and orphan cleanup

**Responsibilities:**
- Create, start, stop, and destroy Fly Machines
- Handle machine state transitions and lifecycle events
- Implement port mapping and network configuration
- Stream machine logs and I/O via Machines API
- Manage machine labels and metadata
- Graceful shutdown and cleanup on session termination

**Key Tasks:**
- Implement machine creation with proper image specification
- Set up port mappings for code execution ports
- Handle machine start/stop state synchronization
- Implement orphan machine detection and cleanup
- Monitor machine resources and limits

**Coordination Notes:**
- Consult with Database Agent before persisting machine state
- Do not create machines until Session Manager confirms schema is ready
- Machine lifecycle timing must align with API responses (wait for readiness)
- Coordinate with API Agent on error responses for machine failures
- All machine create calls should include descriptive labels for tracking

---

## API Agent

**Expertise:** chi router, huma OpenAPI, HTTP handler design

**Owns:**
- `internal/api/handlers.go` - HTTP handler implementations
- `internal/api/server.go` - Server setup and routing
- `internal/api/types.go` - Request/response types with OpenAPI annotations
- `internal/api/openapi.go` - OpenAPI spec generation (if used)

**Responsibilities:**
- Design REST endpoints for session lifecycle (POST/GET/DELETE)
- Request validation and error handling
- Response formatting and status codes
- HTTP middleware integration point
- OpenAPI spec generation and documentation
- Session creation with machine provisioning
- Machine start/stop/destroy endpoints

**Key Tasks:**
- POST /v1/sessions - Create session with code and Docker image
- GET /v1/sessions/{id} - Fetch session details
- GET /v1/sessions - List sessions with filtering
- DELETE /v1/sessions/{id} - Terminate session and cleanup
- Handle edge cases: invalid images, quota exceeded, Fly API errors

**Coordination Notes:**
- All request types must have OpenAPI annotations
- Do not call Backend Agent directly; use Session Manager abstraction
- Coordinate error handling with Auth Agent for auth failures vs app errors
- WebSocket Agent owns the /v1/sessions/{id}/attach endpoint
- All handlers must accept context.Context as first parameter
- Validate rate limit headers from Auth Agent before processing

---

## WebSocket Agent

**Expertise:** gorilla/websocket, binary protocols, concurrent I/O, goroutine management

**Owns:**
- `internal/api/websocket.go` - WebSocket connection handling
- `internal/proto/protocol.go` - Binary protocol definitions
- `internal/proto/framing.go` - Message framing and serialization
- `internal/api/attach.go` - Attach endpoint logic

**Responsibilities:**
- Implement binary protocol for code execution streams
- Handle WebSocket upgrades and connection lifecycle
- Multiplex stdin/stdout/stderr over single connection
- Manage concurrent read/write operations safely
- Handle connection drops and reconnection logic
- Implement proper backpressure and buffering

**Protocol Pattern:**
```
Client connects: GET /v1/sessions/{id}/attach
Upgrade to WebSocket
Client sends: { type: "stdin", data: [...] }
Server sends: { type: "stdout", data: [...] }
Server sends: { type: "exit_code", code: 0 }
Connection closes
```

**Key Responsibilities:**
- Maintain exactly 4 goroutines per connection: read, write, stdin flush, stdout/stderr multiplex
- Implement proper cleanup on connection drop
- Handle message framing with length prefixes
- Validate session ownership before attaching
- Send exit codes and process termination signals

**Coordination Notes:**
- Consult with Auth Agent to validate session access before upgrade
- Coordinate with Backend Agent for I/O stream management
- Do not hold locks during I/O operations (causes deadlocks)
- All message types must be defined in protocol.go
- Exit code signals must match those from backend machines

---

## Auth Agent

**Expertise:** Middleware, rate limiting, API key validation, request logging

**Owns:**
- `internal/api/middleware.go` - All authentication and authorization middleware
- `internal/api/ratelimit.go` - Rate limiting implementation
- `internal/api/logging.go` - Request/response logging

**Responsibilities:**
- Bearer token validation (API key format)
- API key lookup and user association
- Rate limiting per API key and per IP
- Request logging with structured fields
- Error handling for auth failures (401, 429 responses)
- Session ownership validation

**Key Tasks:**
- Implement Bearer token extraction and validation
- Rate limit: 100 requests/minute per API key
- Rate limit: 10 concurrent machines per API key
- Log failed auth attempts
- Validate session ownership before allowing operations

**Authentication Flow:**
```
Request arrives with Authorization: Bearer <api_key>
1. Extract API key from header
2. Look up in database (api_keys table)
3. Check if active and not revoked
4. Attach user_id to context
5. Continue to handler (or return 401)
```

**Coordination Notes:**
- Do not modify api_keys table schema without Database Agent
- WebSocket Agent must validate session ownership before upgrade
- Rate limit headers should be included in all responses
- Failed auth should not leak information about whether key exists
- Coordinate with Database Agent on efficient key lookup queries

---

## Infrastructure Agent

**Expertise:** Fly.io deployment, environment config, CI/CD, secrets management

**Owns:**
- `fly.toml` - Fly.io deployment configuration
- `Makefile` - Build and deployment targets
- `deploy/` - Deployment scripts and utilities
- `.env.example` - Environment variable documentation
- GitHub Actions workflows (if present)

**Responsibilities:**
- Fly.io app configuration and regions
- Environment variables and secrets management
- Build and deploy automation
- Database connection pooling in production
- Monitoring and logging configuration
- Zero-downtime deployment strategy

**Configuration:**
```
fly.toml:
- app name and region
- internal and exposed ports
- environment variables (non-secret)
- health check endpoint
- resource limits (memory, CPU)

.env variables:
- DATABASE_URL (Supabase connection)
- FLY_API_TOKEN (Machines API access)
- LOG_LEVEL (debug/info/warn/error)
```

**Key Tasks:**
- Set up secrets in Fly (API tokens, database URLs)
- Configure health checks for readiness/liveness
- Set up structured logging to Fly Logs
- Configure auto-scaling policy
- Enable monitoring for machine creation latency

**Coordination Notes:**
- Consult with Backend Agent before changing port mappings
- Consult with Database Agent before changing connection pooling settings
- Environment variables must be documented in .env.example
- Never hardcode secrets in fly.toml (use secrets only)
- All deployments require explicit approval before pushing

---

## Coordination Rules

**When Adding Features:**
1. Database Agent designs schema first
2. Backend Agent implements Fly API integration
3. API Agent builds HTTP handlers
4. WebSocket Agent handles streaming (if needed)
5. Auth Agent adds access control
6. Infrastructure Agent updates deployment config

**Boundary Violations (Anti-Patterns):**
- Do not call Fly API directly from handlers (use Backend Agent's client)
- Do not execute SQL in handlers (use Database Agent's query methods)
- Do not mix authentication with business logic
- Do not change WebSocket protocol without all agents agreeing
- Do not commit schema changes without migration files

**Testing Expectations:**
- Database Agent: All migrations must be reversible and tested
- Backend Agent: Mock Fly API responses for unit tests
- API Agent: Test handlers with injected dependencies
- WebSocket Agent: Test frame boundaries and concurrent writes
- Auth Agent: Test all edge cases (missing keys, revoked keys, rate limits)

---

## File Structure

| What | Who | Where |
|------|-----|-------|
| Database migrations | DB Agent | `internal/db/migrations/*.sql` |
| Query methods | DB Agent | `internal/db/models.go` |
| DB client setup | DB Agent | `internal/db/client.go` |
| Fly API client | Backend Agent | `internal/backend/fly/client.go` |
| Machine lifecycle | Backend Agent | `internal/backend/fly/machines.go` |
| HTTP handlers | API Agent | `internal/api/handlers.go` |
| Router setup | API Agent | `internal/api/server.go` |
| Request types | API Agent | `internal/api/types.go` |
| WebSocket logic | WS Agent | `internal/api/websocket.go` |
| Binary protocol | WS Agent | `internal/proto/protocol.go` |
| Auth middleware | Auth Agent | `internal/api/middleware.go` |
| Rate limiting | Auth Agent | `internal/api/ratelimit.go` |
| Fly.io config | Infra Agent | `fly.toml` |
| Secrets & vars | Infra Agent | `.env.example` |
| Build targets | Infra Agent | `Makefile` |
| Main entrypoint | Infra Agent | `cmd/server/main.go` |

---

## Integration Test Runner

The script `scripts/run-integration-tests.sh` automates the full test workflow:

```bash
# Run all tests (unit + integration) with full backend
./scripts/run-integration-tests.sh
```

**What it does:**
1. Checks prerequisites (Docker, Go, microk8s kubeconfig)
2. Starts development database (PostgreSQL on port 5433)
3. Starts server with K8s backend in background
4. Waits for health check at `http://localhost:28080/health`
5. Runs unit tests: `go test ./...`
6. Runs integration tests: `go test -tags integration ./test/integration/...`
7. Reports results with pass/fail summary
8. Cleans up server (leaves database running for next run)

**Prerequisites:**
- Docker and Docker Compose installed
- Go installed
- microk8s kubeconfig at `/tmp/microk8s-kubeconfig`
- K8s namespace `execbox` exists with proper RBAC

**Output:**
- Server logs: `/tmp/execbox.log`
- Color-coded pass/fail results
- Exit code 0 on success, 1 on any test failure

**Environment used by script:**
```bash
BACKEND=kubernetes
K8S_KUBECONFIG=/tmp/microk8s-kubeconfig
K8S_NAMESPACE=execbox
DATABASE_URL=postgresql://postgres:postgres@localhost:5433/execbox
PORT=28080
LOG_LEVEL=debug
```

---

## Known Gotchas

1. **Schema changes must be migrations** - Never ALTER TABLE in models.go; create a .sql migration
2. **Machine readiness timing** - Fly returns machine created but not ready; always wait for status
3. **WebSocket frame boundaries** - Binary protocol must include length prefixes to avoid frame loss
4. **Rate limit race** - Use atomic operations for concurrent counter increments
5. **Session cleanup on drop** - If WebSocket closes, signal Backend Agent to stop machine
6. **Connection pooling** - Keep connections alive with TCP keepalive; don't create new pool per request
