# Namespace strategies for MCP Gateway

This document describes different approaches to organizing MCP servers and
related Kubernetes resources across namespaces.

## Overview of resources

When deploying MCP servers with mcp-gateway, you work with these resources:

| Resource | Purpose | Namespace considerations |
|----------|---------|-------------------------|
| Deployment | Runs the MCP server pods | Can be in any namespace |
| Service | Exposes the MCP server internally | Same namespace as Deployment |
| HTTPRoute | Routes traffic to the Service | Can reference cross-namespace Services |
| MCPServer | Configures mcp-gateway integration | References HTTPRoute in same namespace |
| Gateway | Entry point (managed by mcp-gateway) | Usually in `mcp-system` |
| ReferenceGrant | Permits cross-namespace references | In the target namespace |

## Strategy 1: Single namespace (simplest)

Place all resources in one namespace (e.g., `mcp-test`).

```text
mcp-test/
├── moon-server-deployment.yaml
├── moon-server-service.yaml
├── moon-httproute.yaml
├── moon-mcpserver.yaml
├── quotes-server-deployment.yaml
├── quotes-server-service.yaml
├── quotes-httproute.yaml
├── quotes-mcpserver.yaml
└── ...
```

**Pros:**

- Simplest setup, no ReferenceGrant needed
- Easy to manage and debug
- All resources visible with `kubectl get all -n mcp-test`

**Cons:**

- No isolation between services
- All services share the same RBAC policies
- Harder to manage at scale

**Best for:**

- Development and testing
- Small deployments
- Quick prototyping

**Example:**

```yaml
# All in mcp-test namespace
apiVersion: apps/v1
kind: Deployment
metadata:
  name: moon-server
  namespace: mcp-test
---
apiVersion: v1
kind: Service
metadata:
  name: moon-service
  namespace: mcp-test
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: moon-route
  namespace: mcp-test
spec:
  backendRefs:
    - name: moon-service  # Same namespace, no ReferenceGrant needed
      port: 8081
---
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPServer
metadata:
  name: moon-mcp-server
  namespace: mcp-test
```

## Strategy 2: Separate namespace per service (most isolated)

Each MCP server gets its own namespace with all related resources.

```text
moon-server/
├── namespace.yaml
├── deployment.yaml
├── service.yaml
├── httproute.yaml
└── mcpserver.yaml

quotes-server/
├── namespace.yaml
├── deployment.yaml
├── service.yaml
├── httproute.yaml
└── mcpserver.yaml
```

**Pros:**

- Complete isolation between services
- Independent RBAC per service
- Clear ownership boundaries
- Easy to delete entire service (delete namespace)

**Cons:**

- More namespaces to manage
- HTTPRoutes in different namespaces may complicate gateway configuration
- More verbose resource definitions

**Best for:**

- Multi-team environments
- Services with different security requirements
- Production deployments with strict isolation needs

**Example:**

```yaml
# moon-server namespace
apiVersion: v1
kind: Namespace
metadata:
  name: moon-server
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: moon-server
  namespace: moon-server
---
apiVersion: v1
kind: Service
metadata:
  name: moon-service
  namespace: moon-server
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: moon-route
  namespace: moon-server
  labels:
    mcp-server: "true"
spec:
  parentRefs:
    - name: mcp-gateway
      namespace: mcp-system
  backendRefs:
    - name: moon-service  # Same namespace
      port: 8081
---
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPServer
metadata:
  name: moon-mcp-server
  namespace: moon-server
```

## Strategy 3: Hybrid (backend services separate, gateway resources together)

Backend Deployments and Services in separate namespaces, but HTTPRoutes and
MCPServer resources in a shared namespace. **This requires ReferenceGrant.**

```text
moon-server/           # Backend only
├── namespace.yaml
├── deployment.yaml
├── service.yaml
└── referencegrant.yaml  # Allows mcp-test to reference this service

quotes-server/         # Backend only
├── namespace.yaml
├── deployment.yaml
├── service.yaml
└── referencegrant.yaml

mcp-test/              # Gateway resources
├── namespace.yaml
├── moon-httproute.yaml
├── moon-mcpserver.yaml
├── quotes-httproute.yaml
└── quotes-mcpserver.yaml
```

**Pros:**

- Backend isolation (different teams can own different backends)
- Centralized gateway configuration
- Clear separation of concerns
- Gateway team manages routes, backend teams manage services

**Cons:**

- Requires ReferenceGrant for each backend namespace
- More complex setup
- Cross-namespace debugging can be harder

**Best for:**

- Organizations with separate platform and application teams
- When gateway configuration needs central management
- When backends have different lifecycle requirements

**Example:**

```yaml
# In moon-server namespace: ReferenceGrant allowing mcp-test to reference
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-mcp-test-httproutes
  namespace: moon-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
  to:
    - group: ""
      kind: Service
---
# In mcp-test namespace: HTTPRoute referencing cross-namespace service
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: moon-route
  namespace: mcp-test
  labels:
    mcp-server: "true"
spec:
  parentRefs:
    - name: mcp-gateway
      namespace: mcp-system
  backendRefs:
    - name: moon-service
      namespace: moon-server  # Cross-namespace reference
      port: 8081
```

## ReferenceGrant deep dive

### When ReferenceGrant is required

ReferenceGrant is needed when:

1. HTTPRoute references a Service in a different namespace
1. Gateway references a Secret in a different namespace
1. Any Gateway API resource references another resource across namespaces

### ReferenceGrant placement

**Important:** ReferenceGrant must be in the **target** namespace (where the
referenced resource lives), not the source namespace.

```text
Source namespace (mcp-test)     Target namespace (moon-server)
┌─────────────────────────┐     ┌─────────────────────────────┐
│ HTTPRoute               │     │ Service                     │
│   backendRefs:          │────▶│   name: moon-service        │
│     - name: moon-service│     │                             │
│       namespace: moon-  │     │ ReferenceGrant              │
│                server   │     │   from: mcp-test/HTTPRoute  │
└─────────────────────────┘     │   to: Service               │
                                └─────────────────────────────┘
```

### ReferenceGrant patterns

#### Allow specific namespace

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-mcp-test
  namespace: moon-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test  # Only this namespace
  to:
    - group: ""
      kind: Service
```

#### Allow multiple namespaces

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-multiple-namespaces
  namespace: moon-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: staging
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: production
  to:
    - group: ""
      kind: Service
```

#### Allow specific service only

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-specific-service
  namespace: moon-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
  to:
    - group: ""
      kind: Service
      name: moon-service  # Only this specific service
```

## Recommendations by environment

### Development/Testing

Use **Strategy 1 (Single namespace)**:

```bash
kubectl create namespace mcp-test
kubectl apply -f k8s/mcp-gateway-examples/
```

### Staging

Use **Strategy 2 (Separate namespaces)** or **Strategy 3 (Hybrid)**:

- Separate namespaces help catch permission issues before production
- Practice with ReferenceGrant configuration

### Production

Use **Strategy 2** or **Strategy 3** based on team structure:

- **Strategy 2**: When each service team owns the complete stack
- **Strategy 3**: When a platform team manages gateway configuration

## Common pitfalls

### Missing ReferenceGrant

**Symptom:** HTTPRoute shows `BackendNotFound` or similar error

**Solution:** Create ReferenceGrant in the target namespace

```bash
kubectl get httproute -n mcp-test -o yaml | grep -A5 "status:"
# Look for: "Backend not found" or "RefNotPermitted"
```

### ReferenceGrant in wrong namespace

**Symptom:** Cross-namespace reference still not working

**Solution:** Move ReferenceGrant to the **target** namespace (where the Service
lives), not the source namespace

### MCPServer and HTTPRoute namespace mismatch

**Symptom:** MCPServer can't find HTTPRoute

**Solution:** MCPServer must be in the same namespace as the HTTPRoute it
references

```yaml
# Both must be in the same namespace
apiVersion: mcp.kagenti.com/v1alpha1
kind: MCPServer
metadata:
  namespace: mcp-test  # Same as HTTPRoute
spec:
  targetRef:
    kind: HTTPRoute
    name: moon-route   # Must exist in mcp-test namespace
```

### Gateway parentRef namespace

**Symptom:** HTTPRoute not attached to Gateway

**Solution:** Specify the Gateway namespace in parentRefs

```yaml
spec:
  parentRefs:
    - name: mcp-gateway
      namespace: mcp-system  # Don't forget this!
```

## Quick reference

| Scenario | ReferenceGrant needed? |
|----------|----------------------|
| HTTPRoute and Service in same namespace | No |
| HTTPRoute and Service in different namespaces | Yes |
| MCPServer and HTTPRoute in same namespace | No |
| MCPServer and HTTPRoute in different namespaces | Not supported |
| HTTPRoute referencing Gateway in different namespace | No (Gateway allows by default) |

## Example ReferenceGrant for this repository

If using Strategy 3 with this repository's sample servers:

```yaml
# Apply in each backend namespace
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-mcp-test-httproutes
  namespace: moon-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
  to:
    - group: ""
      kind: Service
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-mcp-test-httproutes
  namespace: quotes-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
  to:
    - group: ""
      kind: Service
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: ReferenceGrant
metadata:
  name: allow-mcp-test-httproutes
  namespace: weather-server
spec:
  from:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      namespace: mcp-test
  to:
    - group: ""
      kind: Service
```
