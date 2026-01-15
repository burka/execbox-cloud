# Execbox-Cloud Roadmap

## Phase 1: Core API + Fly Backend (Current)
- [x] Go module setup and directory structure
- [x] Supabase schema (api_keys, sessions, usage_metrics)
- [x] Database layer (postgres.go, models.go)
- [x] Fly.io Machines client (create, start, stop, destroy)
- [x] HTTP handlers (POST/GET/DELETE /v1/sessions)
- [x] WebSocket attach endpoint with binary protocol
- [x] API key auth middleware
- [x] Rate limiting
- [ ] Wire Fly Machine I/O to WebSocket attach
- [ ] Add file upload/download endpoints
- [ ] Add /v1/sessions/{id}/url endpoint
- [ ] Integration tests with real Fly.io
- [ ] Deploy to Fly.io

## Phase 2: Image Building (In Progress)
- [x] Setup command hashing (SHA256-based content addressing)
- [x] Image cache table (002_image_cache.sql)
- [x] Cache hit/miss tracking via DB queries
- [x] Builder interface and wiring in handlers
- [ ] Fly remote builder integration (actual build implementation)
- [ ] Base64 file encoding support

## Phase 3: Dashboard
- [ ] React + shadcn/ui frontend
- [ ] Login with API key
- [ ] Usage graphs
- [ ] Running sessions list
- [ ] API key management

## Phase 4: Pluggable Backend Architecture (Done)
- [x] Add `Destroy()` and `Health()` to execbox Backend interface
- [x] Add `BuildFiles` to execbox Spec for build-time file inclusion
- [x] Implement `fly.Backend` wrapper implementing `execbox.Backend`
- [x] Create type conversion layer (Spec <-> MachineConfig)
- [x] Add ListMachines to fly.Client

## Phase 5: Kubernetes Backend

Full Kubernetes integration with real streaming I/O, port forwarding, and multi-tenancy.

### 5.1 Foundation (Pod Lifecycle)
- [ ] Create `internal/backend/k8s/` package structure
- [ ] Add k8s.io/client-go, k8s.io/api dependencies
- [ ] Implement `BackendConfig` (kubeconfig, namespace, serviceAccount, imagePullSecrets)
- [ ] Implement `NewBackend()` with in-cluster and kubeconfig support
- [ ] Implement `SpecToPod()` conversion (Image, Command, Env, WorkDir, TTY, Resources, Labels)
- [ ] Implement `Run()` with Pod creation and wait for Running phase
- [ ] Implement `Get()` and `List()` with pod status mapping
- [ ] Implement `Stop()` (graceful), `Kill()` (force), `Destroy()` (full cleanup)
- [ ] Implement `Health()` via Discovery().ServerVersion()
- [ ] Write interface compliance tests

### 5.2 Exec and Basic I/O
- [ ] Implement `Exec()` using `remotecommand.Executor` (SPDY/WebSocket)
- [ ] Implement `Wait()` with pod status watching for Succeeded/Failed
- [ ] Capture exit codes from container termination status
- [ ] Add init container support for `spec.Setup` commands
- [ ] Add ConfigMap support for `spec.BuildFiles`
- [ ] Integration tests for exec with exit codes

### 5.3 Real Streaming I/O (Key Differentiator)
- [ ] Implement `stdinPipe` with buffering for detached state
- [ ] Implement streaming attachment via `remotecommand.StreamWithContext()`
- [ ] Wire `Stdin()`, `Stdout()`, `Stderr()` to SPDY streams
- [ ] Implement `Attach()` for session reconnection
- [ ] Handle TTY mode with `TerminalSizeQueue` for resize
- [ ] Add WebSocket fallback for proxy compatibility
- [ ] Integration tests for interactive streaming and detach/reattach

### 5.4 Port Forwarding
- [ ] Implement `portforward.PortForwarder` wrapper
- [ ] Implement `URL(port)` with dynamic local port allocation
- [ ] Track active forwarders per Handle
- [ ] Cleanup forwarders on session destroy
- [ ] Integration tests for port forwarding

### 5.5 Volumes and Session Persistence
- [ ] Implement PVC creation for `spec.Volumes`
- [ ] Store session state in Pod annotations (session-id, spec, created-at)
- [ ] Implement session recovery on backend restart
- [ ] Implement garbage collection for stale pods/PVCs
- [ ] Integration tests for volume persistence

### 5.6 Multi-Tenancy (BYOK8S)
- [ ] Implement namespace-per-tenant model
- [ ] Create RBAC manifests (ServiceAccount, ClusterRole, ClusterRoleBinding)
- [ ] Implement ResourceQuota per tenant (CPU, memory, pod count)
- [ ] Implement NetworkPolicy for namespace isolation
- [ ] Add tenant-aware backend configuration
- [ ] Document RBAC requirements for BYOK8S users

### 5.7 Production Hardening
- [ ] Add Prometheus metrics (latency, errors, active sessions)
- [ ] Add structured logging with session context
- [ ] Implement circuit breaker for K8s API calls
- [ ] Implement retry with exponential backoff
- [ ] Create Helm chart for deployment
- [ ] Write setup guide and configuration reference

## Phase 6: Additional Backends (Future)
- [ ] Firecracker backend (microVMs)
- [ ] Self-hosted Docker backend

## Backlog
- [ ] WebSocket streaming for exec
- [ ] File upload/download endpoints
- [ ] Multi-region deployment
- [ ] Billing integration
- [ ] OAuth login
