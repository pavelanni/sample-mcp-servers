# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build commands

### Go servers (local development)

```bash
# Build all servers
cd moon-server && go build -o ../bin/moon-server . && cd ..
cd quotes-server && go build -o ../bin/quotes-server . && cd ..
cd weather-server && go build -o ../bin/weather-server . && cd ..

# Run individual server (default ports: 8081, 8082, 8083)
./bin/moon-server
./bin/quotes-server
./bin/weather-server

# Custom port
./bin/moon-server -port 9001
```

### Container images and Kubernetes

```bash
# Full workflow (Kind + Podman on macOS)
./scripts/setup-kind-with-registry.sh  # Create cluster with local registry
./scripts/build-images.sh               # Build container images
./scripts/push-images.sh                # Push to registry
./scripts/deploy.sh                     # Deploy to cluster

# Cleanup
KIND_EXPERIMENTAL_PROVIDER=podman kind delete cluster --name mcp-cluster
podman rm -f kind-registry
podman volume rm kind-registry-data
```

## Architecture

This project provides three MCP (Model Context Protocol) servers for testing MCP gateways with StreamableHTTP transport.

### Server structure

Each server follows the same pattern:

- **main.go**: Single-file implementation with tool handlers
- **Dockerfile**: Multi-stage build (golang:1.23-alpine → alpine:latest)
- **k8s/**: Namespace, Deployment, Service manifests

### MCP implementation pattern

```go
// 1. Define input/output structs with jsonschema comments
type ToolInput struct {
    Field string `json:"field" jsonschema:"description=...,required"`
}

// 2. Create handler with signature
func handler(ctx context.Context, req *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, OutputType, error)

// 3. Register with MCP server
server := mcp.NewServer(&mcp.Implementation{Name: "...", Version: "1.0.0"}, nil)
mcp.AddTool(server, &mcp.Tool{Name: "...", Description: "..."}, handler)
handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)
```

### Registry architecture (Kind + Podman)

```
Host machine                    Kind cluster
─────────────                   ────────────
localhost:5000  ──────────────► kind-registry:5000 (Podman container)
(push images)                   (pods pull from here)
```

Images are pushed as `localhost:5000/image:tag` but deployments reference `kind-registry:5000/image:tag` because they share the same Podman network.

### Service endpoints (in-cluster)

| Server | URL |
|--------|-----|
| moon | `http://moon-service.moon-server.svc.cluster.local:8081/mcp` |
| quotes | `http://quotes-service.quotes-server.svc.cluster.local:8082/mcp` |
| weather | `http://weather-service.weather-server.svc.cluster.local:8083/mcp` |

## External dependencies

- **Go SDK**: `github.com/modelcontextprotocol/go-sdk` (only external dependency)
- **Open-Meteo API**: Used by weather-server, no API key required
- **ZenQuotes API**: Used by quotes-server with local fallback database
