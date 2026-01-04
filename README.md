# execbox-cloud

Managed execution platform for execbox with multi-backend support.

## Overview

execbox-cloud implements the [execbox remote protocol](https://github.com/burka/execbox/blob/main/docs/remote-protocol.md) as a scalable, hosted service. It enables running ephemeral containers on Fly.io Machines with persistent state tracked in Supabase Postgres.

## Architecture

```
Users/SDKs → execbox-cloud API (Fly.io) → Supabase Postgres
                     ↓
            Fly.io Machines (ephemeral execution)
```

The service exposes HTTP and WebSocket endpoints for session management and process I/O, backed by a fleet of ephemeral Fly.io Machines that spin up on-demand for execution.

## Features

- **Implements execbox remote protocol**: Full HTTP and WebSocket support as specified in remote-protocol.md
- **Fly.io Machines backend**: Ephemeral container execution with automatic cleanup
- **API key authentication**: Bearer token authentication with rate limiting per key
- **Usage tracking**: Metrics collection for execution time, data transfer, and resource consumption
- **Image caching**: Content-addressed builds for faster session startup
- **Session lifecycle management**: Create, monitor, stop, and kill sessions
- **File I/O**: Upload, download, and list files in running sessions
- **Interactive execution**: Exec into running sessions with streaming I/O
- **Port exposure**: Expose and access container ports via HTTP URLs

## Quick Start

### Prerequisites

- Go 1.25+
- Environment variables configured

### Environment Variables

```bash
DATABASE_URL=postgresql://user:password@host/dbname
FLY_API_TOKEN=<your-fly-io-api-token>
FLY_APP_NAME=execbox-cloud
```

### Running

```bash
go run cmd/server/main.go
```

The server starts on the configured port (default: 8080) and exposes:

```
http://localhost:8080/health
http://localhost:8080/v1/sessions
```

## API Endpoints

All endpoints require `Authorization: Bearer <api-key>` header.

### Session Management

**Create Session**
```
POST /v1/sessions
Content-Type: application/json

{
  "image": "python:3.12",
  "command": ["python", "-c", "print('hello')"],
  "env": {"DEBUG": "1"},
  "workDir": "/app",
  "resources": {
    "cpuMillis": 1000,
    "memoryMB": 512,
    "timeoutMs": 30000
  },
  "network": "none|outgoing|exposed",
  "ports": [{"container": 8080, "protocol": "tcp"}]
}

201 Created
{
  "id": "sess_abc123",
  "status": "running",
  "createdAt": "2024-01-15T10:30:00Z",
  "network": {
    "mode": "exposed",
    "host": "localhost",
    "ports": {
      "8080": {"hostPort": 32789, "url": "http://localhost:32789"}
    }
  }
}
```

**Get Session**
```
GET /v1/sessions/{id}

200 OK
{session info}
```

**List Sessions**
```
GET /v1/sessions?status=running

200 OK
{
  "sessions": [{session info}, ...]
}
```

**Stop Session (graceful)**
```
POST /v1/sessions/{id}/stop

200 OK
{"status": "stopped"}
```

**Kill Session (force)**
```
DELETE /v1/sessions/{id}

200 OK
{"status": "killed"}
```

### Process I/O

**Attach to Main Process (WebSocket)**
```
GET /v1/sessions/{id}/attach?protocol=binary

Upgrade: websocket

Binary protocol with message types:
  0x01 - Stdin  (client → server)
  0x02 - Stdout (server → client)
  0x03 - Stderr (server → client)
  0x04 - Exit   (server → client)
  0x05 - Error  (server → client)
  0x06 - StdinClose (client → server)
```

**Interactive Exec (WebSocket)**
```
GET /v1/sessions/{id}/exec?cmd=bash&workdir=/app&protocol=binary

Upgrade: websocket

Same message protocol as attach endpoint.
```

### File Operations

**Upload File**
```
POST /v1/sessions/{id}/files?path=/app/data.csv
Content-Type: application/octet-stream

<file-content-bytes>

201 Created
{"path": "/app/data.csv", "size": 1234}
```

**Download File**
```
GET /v1/sessions/{id}/files?path=/app/output.txt

200 OK
Content-Type: application/octet-stream
<file-content-bytes>
```

**List Directory**
```
GET /v1/sessions/{id}/files?path=/app&list=true

200 OK
{
  "path": "/app",
  "entries": [
    {"name": "main.py", "size": 1234, "isDir": false, "mode": 420},
    {"name": "data", "size": 0, "isDir": true, "mode": 493}
  ]
}
```

**Get Port URL**
```
GET /v1/sessions/{id}/url?port=8080

200 OK
{
  "containerPort": 8080,
  "hostPort": 32789,
  "url": "http://localhost:32789",
  "protocol": "tcp"
}
```

## Error Handling

All errors return JSON with status code and error code:

```json
{
  "error": "session not found",
  "code": "NOT_FOUND"
}
```

Status codes:
- `400 BAD_REQUEST` - Invalid request body or parameters
- `401 UNAUTHORIZED` - Missing or invalid auth token
- `404 NOT_FOUND` - Session or file not found
- `409 CONFLICT` - Session already stopped
- `500 INTERNAL` - Server error

## Tech Stack

- **Language**: Go 1.25+
- **HTTP Framework**: chi
- **API Documentation**: huma
- **WebSocket**: gorilla/websocket
- **Database**: pgx (PostgreSQL driver)
- **Backend**: Fly.io Machines API
- **Database Service**: Supabase Postgres

## Development

### Build

```bash
go build -o bin/execbox-cloud ./cmd/server
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

## Deployment

This service is designed to run on Fly.io. Configuration and deployment instructions are in the Fly.io dashboard.

## References

- [execbox remote protocol specification](https://github.com/flob/execbox/blob/main/docs/remote-protocol.md)
- [Fly.io Machines API](https://fly.io/docs/machines/)
- [Supabase PostgreSQL](https://supabase.com/)

## License

See LICENSE file in repository root.
