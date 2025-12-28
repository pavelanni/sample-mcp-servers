# Kind cluster setup for MCP servers

This document describes how to run the MCP servers in a local Kind-based Kubernetes cluster using Podman on macOS.

## Quick start

```bash
# 1. Create cluster with registry
./scripts/setup-kind-with-registry.sh

# 2. Build images
./scripts/build-images.sh

# 3. Push to registry
./scripts/push-images.sh

# 4. Deploy
./scripts/deploy.sh
```

## How it works

The solution uses an **external registry container** connected to the Kind network:

```
┌──────────────────────────────────────────────────────────────┐
│                   Podman "kind" network                      │
│                                                              │
│  ┌──────────────────┐      ┌───────────────────────────────┐ │
│  │  kind-registry   │◄────►│  mcp-cluster-control-plane    │ │
│  │  (container)     │      │                               │ │
│  │                  │      │  Pods reference images as:    │ │
│  │  Port 5000       │      │  kind-registry:5000/image:tag │ │
│  └────────┬─────────┘      └───────────────────────────────┘ │
└───────────┼──────────────────────────────────────────────────┘
            │ port mapping (127.0.0.1:5000)
            ▼
      localhost:5000  ← host pushes images here
```

Key points:

- **Registry runs as a Podman container** (not inside Kubernetes)
- **Connected to the `kind` network** so Kind nodes can reach it
- **Host pushes to** `localhost:5000/image:tag`
- **Pods pull from** `kind-registry:5000/image:tag`
- Same registry, different hostnames for different network contexts

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/) installed
- [Podman](https://podman.io/) installed and running on macOS
- kubectl configured

## Cleanup

```bash
# Delete cluster
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name mcp-cluster

# Remove registry container and volume
podman rm -f kind-registry
podman volume rm kind-registry-data
```

## Troubleshooting

### Images not pulling (ImagePullBackOff)

1. Check that the registry is connected to the Kind network:

   ```bash
   podman network inspect kind | grep kind-registry
   ```

1. Check containerd configuration on the Kind node:

   ```bash
   podman exec mcp-cluster-control-plane cat /etc/containerd/certs.d/kind-registry:5000/hosts.toml
   ```

   Should show:

   ```toml
   server = "http://kind-registry:5000"

   [host."http://kind-registry:5000"]
     capabilities = ["pull", "resolve", "push"]
   ```

1. Verify images exist in registry:

   ```bash
   curl http://localhost:5000/v2/_catalog
   ```

1. Test registry connectivity from Kind node:

   ```bash
   podman exec mcp-cluster-control-plane curl http://kind-registry:5000/v2/_catalog
   ```

### Registry not accessible from host

Check that the registry container is running and port-mapped:

```bash
podman ps | grep kind-registry
curl http://localhost:5000/v2/
```

## Alternative approaches

### Option 1: `kind load docker-image` (simplest, no registry needed)

Kind can load images directly from Podman into cluster nodes:

```bash
# Build image
podman build -t moon-server:v1.0.0 moon-server/

# Load into Kind (requires saving to tar first with Podman)
podman save moon-server:v1.0.0 -o /tmp/moon-server.tar
kind load image-archive /tmp/moon-server.tar --name mcp-cluster

# Use in deployment (no registry prefix)
# image: moon-server:v1.0.0
```

**Pros:**

- No registry needed
- Simple for small number of images

**Cons:**

- Slower for many images (saves/loads tar files)
- Images must be reloaded after cluster recreation
- Doesn't simulate production registry workflow

### Option 2: Public registry (Docker Hub, ghcr.io)

Push images to a public or private cloud registry:

```bash
# Login
podman login ghcr.io

# Build and push
podman build -t ghcr.io/username/moon-server:v1.0.0 moon-server/
podman push ghcr.io/username/moon-server:v1.0.0

# Use in deployment
# image: ghcr.io/username/moon-server:v1.0.0
# imagePullSecrets may be needed for private repos
```

**Pros:**

- TLS/authentication built-in
- Images persist across cluster recreations
- Closest to production workflow

**Cons:**

- Requires internet connectivity
- Rate limits (Docker Hub)
- Not fully self-contained

### Option 3: Registry with self-signed TLS

Run a local registry with TLS for a more production-like setup:

```bash
# Generate self-signed cert
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout registry.key -out registry.crt \
  -subj "/CN=kind-registry" \
  -addext "subjectAltName=DNS:kind-registry,DNS:localhost"

# Run registry with TLS
podman run -d --name kind-registry \
  -p 5000:5000 \
  -v ./registry.crt:/certs/registry.crt:ro \
  -v ./registry.key:/certs/registry.key:ro \
  -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt \
  -e REGISTRY_HTTP_TLS_KEY=/certs/registry.key \
  registry:2

# Configure Kind nodes to trust the cert
# (copy cert to /etc/containerd/certs.d/kind-registry:5000/ca.crt)
```

**Pros:**

- More production-like (TLS)
- Can test certificate handling

**Cons:**

- More complex setup
- Certificate management overhead
- Overkill for local development

### Option 4: In-cluster registry with NodePort

Run the registry inside Kubernetes with NodePort exposure:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: registry
spec:
  type: NodePort
  ports:
  - port: 5000
    nodePort: 30500
  selector:
    app: registry
```

**Pros:**

- Registry managed by Kubernetes
- Can use persistent volumes

**Cons:**

- Chicken-and-egg: need images to deploy registry
- Port-forward/NodePort networking issues with Podman on macOS
- More complex containerd configuration

## Recommendation

For local development on macOS with Podman, the **external registry on Kind network** (current solution) provides the best balance:

| Criteria | External Registry | kind load | Public Registry | In-cluster |
|----------|------------------|-----------|-----------------|------------|
| Setup complexity | Medium | Low | Low | High |
| Self-contained | Yes | Yes | No | Yes |
| Production-like | Yes | No | Yes | Yes |
| Works with Podman/macOS | Yes | Yes | Yes | Difficult |
| Fast iteration | Yes | No | Yes | Yes |

For CI/CD or team environments, use a **public/private cloud registry** (ghcr.io, ECR, GCR) for proper authentication, TLS, and image persistence.
