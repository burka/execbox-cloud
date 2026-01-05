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

## Phase 4: Additional Backends (Future)
- [ ] Kubernetes backend
- [ ] Firecracker backend
- [ ] Self-hosted Docker backend

## Backlog
- [ ] WebSocket streaming for exec
- [ ] File upload/download
- [ ] Multi-region deployment
- [ ] Billing integration
- [ ] OAuth login
