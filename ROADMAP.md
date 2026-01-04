# Execbox-Cloud Roadmap

## Phase 1: Core API + Fly Backend (Current)
- [ ] Go module setup and directory structure
- [ ] Supabase schema (api_keys, sessions, usage_metrics)
- [ ] Database layer (postgres.go, models.go)
- [ ] Fly.io Machines client (create, start, stop, destroy)
- [ ] HTTP handlers (POST/GET/DELETE /v1/sessions)
- [ ] WebSocket attach endpoint with binary protocol
- [ ] API key auth middleware
- [ ] Rate limiting
- [ ] Deploy to Fly.io

## Phase 2: Image Building
- [ ] Setup command hashing
- [ ] Fly remote builder integration
- [ ] Image cache table
- [ ] Cache hit/miss tracking

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
