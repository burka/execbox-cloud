#!/bin/bash
set -euo pipefail

# ============================================================================
# Integration Test Runner for execbox-cloud
# ============================================================================
# This script orchestrates the full integration test workflow:
# 1. Starts development database (if not running)
# 2. Starts server with K8s backend in background
# 3. Runs unit tests
# 4. Runs integration tests
# 5. Cleans up and reports results
# ============================================================================

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SERVER_PID_FILE="/tmp/execbox-server.pid"
SERVER_LOG_FILE="/tmp/execbox.log"
SERVER_PORT=28080
DB_CHECK_TIMEOUT=60
SERVER_CHECK_TIMEOUT=30
K8S_KUBECONFIG="/tmp/microk8s-kubeconfig"
K8S_NAMESPACE="execbox"
DATABASE_URL="postgresql://postgres:postgres@localhost:5433/execbox"
DOCKER_COMPOSE="docker compose"  # Will be set during prereq check

# Test result tracking
UNIT_TEST_EXIT_CODE=0
INTEGRATION_TEST_EXIT_CODE=0
SERVER_STARTED=0

# ============================================================================
# Helper Functions
# ============================================================================

timestamp() {
    date '+%Y-%m-%d %H:%M:%S'
}

log_info() {
    echo -e "${BLUE}[$(timestamp)]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[$(timestamp)] ✓${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[$(timestamp)] ⚠${NC} $1"
}

log_error() {
    echo -e "${RED}[$(timestamp)] ✗${NC} $1"
}

log_section() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}========================================${NC}"
}

# ============================================================================
# Cleanup Handler
# ============================================================================

cleanup() {
    local exit_code=$?

    log_section "Cleanup"

    # Stop server if we started it
    if [ -f "${SERVER_PID_FILE}" ]; then
        local pid=$(cat "${SERVER_PID_FILE}")
        if ps -p "${pid}" > /dev/null 2>&1; then
            log_info "Stopping server (PID: ${pid})..."
            kill "${pid}" 2>/dev/null || true
            sleep 2
            # Force kill if still running
            if ps -p "${pid}" > /dev/null 2>&1; then
                log_warn "Server still running, force killing..."
                kill -9 "${pid}" 2>/dev/null || true
            fi
            log_success "Server stopped"
        fi
        rm -f "${SERVER_PID_FILE}"
    fi

    # Note: We intentionally leave the database running for next test run
    log_info "Database left running for next test run"
    log_info "To stop database: make stop-devdb"

    return $exit_code
}

trap cleanup EXIT INT TERM

# ============================================================================
# Prerequisite Checks
# ============================================================================

check_prerequisites() {
    log_section "Checking Prerequisites"

    local all_ok=1

    # Check Docker
    if command -v docker &> /dev/null; then
        log_success "Docker found"
    else
        log_error "Docker not found - required for database"
        all_ok=0
    fi

    # Check Docker Compose (plugin or standalone)
    if docker compose version &> /dev/null; then
        DOCKER_COMPOSE="docker compose"
        log_success "Docker Compose found (plugin)"
    elif command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE="docker-compose"
        log_success "Docker Compose found (standalone)"
    else
        log_error "Docker Compose not found - required for database"
        all_ok=0
    fi

    # Check Go
    if command -v go &> /dev/null; then
        local go_version=$(go version | awk '{print $3}')
        log_success "Go found (${go_version})"
    else
        log_error "Go not found - required for running tests"
        all_ok=0
    fi

    # Check curl
    if command -v curl &> /dev/null; then
        log_success "curl found"
    else
        log_error "curl not found - required for health checks"
        all_ok=0
    fi

    # Check microk8s kubeconfig
    if [ -f "${K8S_KUBECONFIG}" ]; then
        log_success "Kubernetes config found: ${K8S_KUBECONFIG}"
    else
        log_error "Kubernetes config not found: ${K8S_KUBECONFIG}"
        log_info "Create it with: microk8s config > ${K8S_KUBECONFIG}"
        all_ok=0
    fi

    # Check if project root contains go.mod
    if [ -f "${PROJECT_ROOT}/go.mod" ]; then
        log_success "Project root found: ${PROJECT_ROOT}"
    else
        log_error "go.mod not found in project root: ${PROJECT_ROOT}"
        all_ok=0
    fi

    if [ $all_ok -eq 0 ]; then
        log_error "Prerequisites check failed"
        exit 1
    fi

    log_success "All prerequisites satisfied"
}

# ============================================================================
# Database Management
# ============================================================================

check_database_running() {
    # Check via docker-compose first
    ${DOCKER_COMPOSE} -f "${PROJECT_ROOT}/docker-compose.yml" ps postgres 2>/dev/null | grep -q "Up" && return 0
    # Also check directly for container by name (handles containers from other projects)
    docker ps --filter name=execbox-postgres --format "{{.Status}}" 2>/dev/null | grep -q "Up" && return 0
    return 1
}

wait_for_database() {
    log_info "Waiting for database to be ready (timeout: ${DB_CHECK_TIMEOUT}s)..."

    local elapsed=0
    while [ $elapsed -lt $DB_CHECK_TIMEOUT ]; do
        # Try docker exec directly on the container (works regardless of how container was started)
        if docker exec execbox-postgres pg_isready -U postgres -d execbox > /dev/null 2>&1; then
            log_success "Database is ready"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done

    echo ""
    log_error "Database did not become ready within ${DB_CHECK_TIMEOUT}s"
    return 1
}

start_database() {
    log_section "Starting Development Database"

    if check_database_running; then
        log_success "Database already running"
        return 0
    fi

    log_info "Starting database with 'make run-devdb'..."
    cd "${PROJECT_ROOT}"
    make run-devdb

    wait_for_database
}

# ============================================================================
# Server Management
# ============================================================================

kill_existing_server() {
    # Kill any process using the server port
    local pid=$(lsof -ti:${SERVER_PORT} 2>/dev/null || true)
    if [ -n "$pid" ]; then
        log_warn "Found existing process on port ${SERVER_PORT} (PID: ${pid})"
        kill "$pid" 2>/dev/null || true
        sleep 2
        # Force kill if still running
        if ps -p "$pid" > /dev/null 2>&1; then
            log_warn "Force killing process ${pid}..."
            kill -9 "$pid" 2>/dev/null || true
        fi
        log_success "Killed existing server"
    fi

    # Clean up old PID file
    if [ -f "${SERVER_PID_FILE}" ]; then
        rm -f "${SERVER_PID_FILE}"
    fi
}

start_server() {
    log_section "Starting Server with Kubernetes Backend"

    # Kill any existing server
    kill_existing_server

    # Clear or create log file
    > "${SERVER_LOG_FILE}"

    log_info "Server configuration:"
    log_info "  BACKEND=kubernetes"
    log_info "  K8S_KUBECONFIG=${K8S_KUBECONFIG}"
    log_info "  K8S_NAMESPACE=${K8S_NAMESPACE}"
    log_info "  DATABASE_URL=${DATABASE_URL}"
    log_info "  PORT=${SERVER_PORT}"
    log_info "  LOG_LEVEL=debug"
    log_info "  Log file: ${SERVER_LOG_FILE}"

    # Start server in background
    cd "${PROJECT_ROOT}"
    BACKEND=kubernetes \
    K8S_KUBECONFIG="${K8S_KUBECONFIG}" \
    K8S_NAMESPACE="${K8S_NAMESPACE}" \
    DATABASE_URL="${DATABASE_URL}" \
    PORT="${SERVER_PORT}" \
    LOG_LEVEL=debug \
    go run ./cmd/server > "${SERVER_LOG_FILE}" 2>&1 &

    local server_pid=$!
    echo "${server_pid}" > "${SERVER_PID_FILE}"
    SERVER_STARTED=1

    log_info "Server started (PID: ${server_pid})"

    # Wait for server to be ready
    wait_for_server
}

wait_for_server() {
    log_info "Waiting for server health check (timeout: ${SERVER_CHECK_TIMEOUT}s)..."

    local elapsed=0
    local health_url="http://localhost:${SERVER_PORT}/health"

    while [ $elapsed -lt $SERVER_CHECK_TIMEOUT ]; do
        if curl -f -s "${health_url}" > /dev/null 2>&1; then
            log_success "Server is healthy"
            return 0
        fi

        # Check if server process is still running
        if [ -f "${SERVER_PID_FILE}" ]; then
            local pid=$(cat "${SERVER_PID_FILE}")
            if ! ps -p "${pid}" > /dev/null 2>&1; then
                log_error "Server process died unexpectedly"
                log_info "Last 20 lines of server log:"
                tail -n 20 "${SERVER_LOG_FILE}" | sed 's/^/  /'
                return 1
            fi
        fi

        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done

    echo ""
    log_error "Server did not become healthy within ${SERVER_CHECK_TIMEOUT}s"
    log_info "Last 20 lines of server log:"
    tail -n 20 "${SERVER_LOG_FILE}" | sed 's/^/  /'
    return 1
}

# ============================================================================
# Test Execution
# ============================================================================

run_unit_tests() {
    log_section "Running Unit Tests"

    cd "${PROJECT_ROOT}"

    log_info "Running: go test ./... -v"

    if go test ./... -v; then
        UNIT_TEST_EXIT_CODE=0
        log_success "Unit tests passed"
        return 0
    else
        UNIT_TEST_EXIT_CODE=$?
        log_error "Unit tests failed (exit code: ${UNIT_TEST_EXIT_CODE})"
        return 1
    fi
}

run_integration_tests() {
    log_section "Running Integration Tests"

    cd "${PROJECT_ROOT}"

    log_info "Running: go test ./test/integration/... -tags integration -v -timeout 5m"

    if go test ./test/integration/... -tags integration -v -timeout 5m; then
        INTEGRATION_TEST_EXIT_CODE=0
        log_success "Integration tests passed"
        return 0
    else
        INTEGRATION_TEST_EXIT_CODE=$?
        log_error "Integration tests failed (exit code: ${INTEGRATION_TEST_EXIT_CODE})"
        return 1
    fi
}

# ============================================================================
# Results Summary
# ============================================================================

print_summary() {
    log_section "Test Results Summary"

    echo ""
    echo -e "${CYAN}Test Results:${NC}"
    echo "─────────────────────────────────────────"

    # Unit tests
    if [ $UNIT_TEST_EXIT_CODE -eq 0 ]; then
        echo -e "Unit Tests:        ${GREEN}✓ PASSED${NC}"
    else
        echo -e "Unit Tests:        ${RED}✗ FAILED${NC} (exit code: ${UNIT_TEST_EXIT_CODE})"
    fi

    # Integration tests
    if [ $INTEGRATION_TEST_EXIT_CODE -eq 0 ]; then
        echo -e "Integration Tests: ${GREEN}✓ PASSED${NC}"
    else
        echo -e "Integration Tests: ${RED}✗ FAILED${NC} (exit code: ${INTEGRATION_TEST_EXIT_CODE})"
    fi

    echo "─────────────────────────────────────────"

    # Overall result
    if [ $UNIT_TEST_EXIT_CODE -eq 0 ] && [ $INTEGRATION_TEST_EXIT_CODE -eq 0 ]; then
        echo -e "${GREEN}Overall: ALL TESTS PASSED ✓${NC}"
        echo ""
        return 0
    else
        echo -e "${RED}Overall: SOME TESTS FAILED ✗${NC}"
        echo ""
        echo "Review logs for details:"
        echo "  Server log: ${SERVER_LOG_FILE}"
        return 1
    fi
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
    log_section "Integration Test Runner"
    log_info "Project: ${PROJECT_ROOT}"
    log_info "Started at: $(timestamp)"

    # Check prerequisites
    check_prerequisites

    # Start database
    start_database

    # Start server
    start_server

    # Run tests (continue even if unit tests fail, to run integration tests)
    run_unit_tests || true
    run_integration_tests || true

    # Print summary
    print_summary

    # Return appropriate exit code
    if [ $UNIT_TEST_EXIT_CODE -eq 0 ] && [ $INTEGRATION_TEST_EXIT_CODE -eq 0 ]; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main
