# Kubernetes Backend Setup Guide

This guide walks you through setting up the Kubernetes backend for execbox-cloud, which allows running interactive sessions and building container images inside a Kubernetes cluster.

## Prerequisites

### 1. Kubernetes Cluster

You need a running Kubernetes cluster. Options include:

- **Local Development**: [minikube](https://minikube.sigs.k8s.io/), [kind](https://kind.sigs.k8s.io/), or [microk8s](https://microk8s.io/)
- **Cloud Provider**: EKS (AWS), GKE (Google Cloud), AKS (Azure)
- **Self-Hosted**: kubeadm, k3s, or other distributions

For testing, microk8s is recommended for simplicity:
```bash
# Install microk8s (Ubuntu/Debian)
sudo snap install microk8s --classic

# Enable required addons
microk8s enable dns
microk8s enable storage

# Start microk8s
microk8s start
```

### 2. kubectl Configuration

Ensure kubectl is configured to access your cluster:

```bash
# Verify cluster access
kubectl cluster-info

# Check current context
kubectl config current-context

# List all contexts
kubectl config get-contexts
```

For in-cluster deployments, execbox-cloud will use the in-cluster kubeconfig automatically. For external clusters, provide the path to your kubeconfig file.

### 3. Container Registry Access

The Kubernetes backend builds container images using Kaniko. You have two options:

- **ttl.sh** (recommended for testing): Anonymous, temporary image registry - images expire after configurable TTL
- **Custom Registry**: Docker Hub, ECR, GCR, or any registry with authentication

For ttl.sh, no configuration is needed. For custom registries, you'll need credentials.

## Quick Start

### 1. Apply Kubernetes Manifests

Deploy the required resources (namespace, RBAC, optional quotas):

```bash
# Using kubectl directly
kubectl apply -k deploy/k8s/

# Or apply individual files
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/rbac.yaml
```

This creates:
- `execbox` namespace
- `execbox` ServiceAccount
- `execbox-controller` Role with necessary permissions
- RoleBinding to grant permissions to the ServiceAccount

Optional: Apply resource quotas:
```bash
kubectl apply -f deploy/k8s/resourcequota.yaml
```

### 2. Verify Installation

```bash
# Check namespace
kubectl get namespace execbox

# Check ServiceAccount
kubectl get serviceaccount execbox -n execbox

# Check RBAC
kubectl get role execbox-controller -n execbox
kubectl get rolebinding execbox-controller -n execbox
```

### 3. Configure Environment Variables

Create a `.env` file or set environment variables:

```bash
# Backend selection
export BACKEND=kubernetes

# Database configuration
export DATABASE_URL=postgresql://postgres:password@localhost:5433/execbox

# Kubernetes configuration
export K8S_KUBECONFIG=/path/to/kubeconfig  # Empty = use in-cluster config
export K8S_NAMESPACE=execbox                # Default: execbox
export K8S_SERVICE_ACCOUNT=execbox          # Default: execbox
export K8S_REGISTRY=ttl.sh                  # Default: ttl.sh
export K8S_IMAGE_TTL=4h                     # Default: 4h

# Server configuration
export PORT=28080
export LOG_LEVEL=info
```

### 4. Start the Server

```bash
go run ./cmd/server
```

The server will:
1. Connect to the Kubernetes cluster
2. Ensure the `execbox` namespace exists
3. Verify RBAC permissions
4. Start listening for requests on the configured port

## Kubernetes Manifest Files

### deploy/k8s/namespace.yaml

Creates the `execbox` namespace where all pods, ConfigMaps, and other resources will be stored.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: execbox
```

### deploy/k8s/rbac.yaml

Defines RBAC permissions for the execbox ServiceAccount:

- **Pod Management**: create, get, list, watch, delete pods
- **Pod Exec**: execute commands in running pods
- **Pod Logs**: read logs from pods
- **Pod Attach**: attach to pod streams
- **Port Forwarding**: forward ports from pods
- **ConfigMaps**: store build files and Dockerfile
- **Services**: expose pod ports (future use)
- **PersistentVolumeClaims**: persistent storage (future use)

The ServiceAccount is used by both the server process (external) and Kaniko build pods (in-cluster).

### deploy/k8s/resourcequota.yaml

Optional resource quotas to limit namespace usage:

- Max 50 pods
- Max 10 CPU cores (requests)
- Max 20 GiB memory (requests)
- Max 20 CPU cores (limits)
- Max 40 GiB memory (limits)
- Max 100 ConfigMaps
- Max 20 PersistentVolumeClaims

Adjust these values based on your cluster capacity and expected workload.

### deploy/k8s/kustomization.yaml

Kustomize configuration for declarative resource management. Includes namespace, RBAC, and optional quota files. Modify to include resourcequota.yaml if needed:

```yaml
resources:
  - namespace.yaml
  - rbac.yaml
  - resourcequota.yaml  # Uncomment to enable quotas
```

## Configuration Options

All K8s-specific environment variables use the `K8S_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKEND` | `fly` | Set to `kubernetes` to use K8s backend |
| `K8S_KUBECONFIG` | (empty) | Path to kubeconfig file. Empty uses in-cluster config or standard kubeconfig locations |
| `K8S_NAMESPACE` | `execbox` | Kubernetes namespace for resources |
| `K8S_SERVICE_ACCOUNT` | `execbox` | ServiceAccount name for RBAC |
| `K8S_REGISTRY` | `ttl.sh` | Container image registry for builds |
| `K8S_IMAGE_TTL` | `4h` | Image TTL on ttl.sh (e.g., `24h`, `7d`) |
| `DATABASE_URL` | (required) | PostgreSQL connection string |
| `PORT` | `28080` | HTTP server port |
| `LOG_LEVEL` | `info` | Logging level: debug, info, warn, error |

### Kubeconfig Resolution

The K8s backend resolves kubeconfig in this order:

1. If `K8S_KUBECONFIG` is set, use that path
2. If running in-cluster, use in-cluster ServiceAccount credentials
3. Otherwise, use default kubeconfig locations:
   - `~/.kube/config`
   - `KUBECONFIG` environment variable

### Registry Configuration

The builder uses these registries:

- **ttl.sh**: Anonymous, no credentials required. Images expire per TTL (default 4h).
  ```
  K8S_REGISTRY=ttl.sh
  K8S_IMAGE_TTL=4h
  ```

- **Docker Hub**:
  ```
  K8S_REGISTRY=docker.io
  K8S_DOCKER_USERNAME=your-username
  K8S_DOCKER_PASSWORD=your-token
  ```

- **ECR** (AWS Elastic Container Registry):
  ```
  K8S_REGISTRY=<account>.dkr.ecr.<region>.amazonaws.com
  ```

- **Custom Registry**:
  ```
  K8S_REGISTRY=registry.example.com
  K8S_REGISTRY_USERNAME=user
  K8S_REGISTRY_PASSWORD=password
  ```

## RBAC Permissions Explained

The `execbox-controller` Role grants the following permissions:

### Core Pod Operations

```yaml
- apiGroups: [""]
  resources: [pods]
  verbs: [get, list, watch, create, delete, deletecollection]
```

Used for:
- Creating session pods
- Monitoring pod status
- Stopping and killing sessions
- Listing active sessions

### Command Execution

```yaml
- apiGroups: [""]
  resources: [pods/exec]
  verbs: [create]
```

Used for executing commands inside running pods via `pod.Exec()`.

### Log Retrieval

```yaml
- apiGroups: [""]
  resources: [pods/log]
  verbs: [get]
```

Used for reading logs from completed pods.

### Stream Attachment

```yaml
- apiGroups: [""]
  resources: [pods/attach]
  verbs: [create]
```

Used for attaching to pod stdin/stdout/stderr streams.

### Port Forwarding

```yaml
- apiGroups: [""]
  resources: [pods/portforward]
  verbs: [create]
```

Reserved for future port forwarding features.

### Build File Storage

```yaml
- apiGroups: [""]
  resources: [configmaps]
  verbs: [get, list, create, delete, deletecollection]
```

Used for:
- Storing build files in ConfigMaps
- Storing Dockerfile for Kaniko builds
- Cleanup of build resources

### Service Management

```yaml
- apiGroups: [""]
  resources: [services]
  verbs: [get, list, create, delete]
```

Reserved for future service exposure features.

### Persistent Storage

```yaml
- apiGroups: [""]
  resources: [persistentvolumeclaims]
  verbs: [get, list, create, delete]
```

Reserved for persistent volume features.

### Minimum Required Permissions

For basic operation, you need:

```yaml
- apiGroups: [""]
  resources: [pods, pods/exec, pods/log, pods/attach]
  verbs: [get, list, watch, create, delete]
- apiGroups: [""]
  resources: [configmaps]
  verbs: [get, list, create, delete, deletecollection]
```

The full set is provided for future expansion and builder support.

## Builder Configuration

The Kubernetes backend builds container images using **Kaniko**, a tool for building container images without Docker daemon access.

### How It Works

1. User provides Dockerfile content and build files
2. Resources are created in a ConfigMap
3. Kaniko pod is created to build the image
4. Image is pushed to the configured registry
5. Pod and ConfigMap are cleaned up automatically

### Image Naming and Caching

Images are named using content-addressed hashing:

```
execbox-<16-char-hash>:<ttl>
```

Example: `ttl.sh/execbox-a1b2c3d4e5f6g7h8:4h`

The hash is computed from:
- Dockerfile content
- Build file contents
- File paths

**Same inputs = same hash = same image name**

This enables caching: if the same Dockerfile and files are built again, Kaniko reuses the existing image from the registry instead of rebuilding.

### ttl.sh Usage

ttl.sh is a free, anonymous Docker-compatible registry:

- **No Authentication**: Anyone can push/pull
- **Automatic Expiration**: Images expire based on TTL tag (e.g., `:4h`, `:24h`, `:7d`)
- **Perfect for Testing**: Low friction for development and CI/CD

```bash
# Expire in 4 hours (default)
ttl.sh/execbox-a1b2c3d4e5f6g7h8:4h

# Expire in 24 hours
ttl.sh/execbox-a1b2c3d4e5f6g7h8:24h

# Expire in 7 days
ttl.sh/execbox-a1b2c3d4e5f6g7h8:7d
```

Set TTL via environment variable:

```bash
export K8S_IMAGE_TTL=24h
```

### Custom Registry Configuration

For production or private registries, create an image pull secret in Kubernetes:

```bash
# Create secret for Docker Hub
kubectl create secret docker-registry regcred \
  --docker-server=docker.io \
  --docker-username=<username> \
  --docker-password=<token> \
  --docker-email=<email> \
  -n execbox

# Or for custom registry
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=<username> \
  --docker-password=<password> \
  -n execbox
```

Then configure:

```bash
export K8S_REGISTRY=registry.example.com/myorg
export K8S_REGISTRY_SECRET=regcred
```

## Running Integration Tests

The Kubernetes backend includes comprehensive integration tests:

```bash
# Run all K8s integration tests with 10 minute timeout
go test -v -tags=integration ./test/integration/... -timeout 10m

# Run specific test
go test -v -tags=integration ./test/integration -run TestK8sBackend -timeout 10m

# With detailed logging
LOG_LEVEL=debug go test -v -tags=integration ./test/integration/... -timeout 10m
```

### Test Requirements

- Kubernetes cluster running and accessible
- `kubectl` configured
- `execbox` namespace created
- RBAC configured
- At least 1 GiB available memory

### Test Coverage

Integration tests verify:
- Pod creation and lifecycle
- Stream attachment (stdin/stdout/stderr)
- Command execution
- Session listing and retrieval
- Graceful stop and kill operations
- Resource cleanup
- Build functionality with Kaniko
- Image caching

## Troubleshooting

### Pod Creation Fails

**Error: "failed to create pod"**

Check RBAC permissions:
```bash
kubectl get rolebinding execbox-controller -n execbox -o yaml
kubectl get role execbox-controller -n execbox -o yaml
```

Verify ServiceAccount:
```bash
kubectl get serviceaccount execbox -n execbox
```

### Permission Denied Errors

**Error: "pods is forbidden"**

The RBAC Role is not properly bound. Verify:

```bash
# Check RoleBinding
kubectl get rolebinding execbox-controller -n execbox

# Verify ServiceAccount is in subjects
kubectl describe rolebinding execbox-controller -n execbox
```

### Kubeconfig Issues

**Error: "failed to load kubeconfig"**

```bash
# Test kubeconfig directly
kubectl --kubeconfig=/path/to/kubeconfig cluster-info

# Verify path and permissions
ls -la /path/to/kubeconfig
```

### Pod Logs Not Available

**Error: "failed to read pod logs"**

Check pod status:
```bash
kubectl get pods -n execbox
kubectl describe pod <pod-name> -n execbox
kubectl logs <pod-name> -n execbox
```

### Build Failures

**Error: "build failed: OOMKilled"**

The Kaniko pod ran out of memory. Increase memory limit in Kaniko pod spec or reduce image complexity.

**Error: "failed to push image"**

Registry credentials missing or incorrect:

```bash
# Check secrets
kubectl get secrets -n execbox

# Create/update secret
kubectl create secret docker-registry regcred \
  --docker-server=<registry> \
  --docker-username=<user> \
  --docker-password=<token> \
  -n execbox --dry-run=client -o yaml | kubectl apply -f -
```

### Debugging Commands

Check pod status:
```bash
kubectl get pods -n execbox
```

Describe a pod:
```bash
kubectl describe pod <pod-name> -n execbox
```

Read pod logs:
```bash
kubectl logs <pod-name> -n execbox
```

Execute command in pod:
```bash
kubectl exec -it <pod-name> -n execbox -- /bin/sh
```

Watch resource usage:
```bash
kubectl top pods -n execbox
```

Check events:
```bash
kubectl get events -n execbox --sort-by='.lastTimestamp'
```

## Environment Variables Summary

### Required

- `DATABASE_URL`: PostgreSQL connection string
- `BACKEND=kubernetes`: Enable Kubernetes backend

### Optional (Defaults Provided)

```bash
K8S_KUBECONFIG=/path/to/kubeconfig     # Default: in-cluster or ~/.kube/config
K8S_NAMESPACE=execbox                   # Default: execbox
K8S_SERVICE_ACCOUNT=execbox             # Default: execbox
K8S_REGISTRY=ttl.sh                     # Default: ttl.sh
K8S_IMAGE_TTL=4h                        # Default: 4h
PORT=28080                              # Default: 28080
LOG_LEVEL=info                          # Default: info
```

## Next Steps

- Deploy execbox-cloud server: `go run ./cmd/server`
- Test with API: `curl http://localhost:28080/health`
- Create sessions via API: POST `/v1/sessions`
- Attach to sessions: WebSocket `/v1/sessions/{id}/attach`
- Build images: POST `/v1/builds`

See [API documentation](./api.md) for endpoint details.
