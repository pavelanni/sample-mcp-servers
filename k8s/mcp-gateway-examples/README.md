# MCP Gateway example manifests

These manifests configure the sample MCP servers to work with
[mcp-gateway](https://github.com/Kuadrant/mcp-gateway).

## Prerequisites

1. mcp-gateway installed in the `mcp-system` namespace
1. Sample MCP servers deployed (moon-server, quotes-server, weather-server)

## Deploy

```bash
# Create the mcp-test namespace
kubectl apply -f namespace.yaml

# Deploy HTTPRoutes and MCPServer resources
kubectl apply -f moon-httproute.yaml -f moon-mcpserver.yaml
kubectl apply -f quotes-httproute.yaml -f quotes-mcpserver.yaml
kubectl apply -f weather-httproute.yaml -f weather-mcpserver.yaml
```

## Verify

```bash
# Check MCPServer status
kubectl get mcpserver -n mcp-test

# Expected output shows discovered tools:
# NAME                 AGE
# moon-mcp-server      1m    # 2 tools
# quotes-mcp-server    1m    # 2 tools
# weather-mcp-server   1m    # 1 tool
```

## Key configuration notes

### HTTPRoute labels

The `mcp-server: "true"` label on HTTPRoute is required for mcp-gateway discovery:

```yaml
metadata:
  labels:
    mcp-server: "true"
```

### MCPServer labels

The `kagenti/mcp: "true"` label on MCPServer is recommended:

```yaml
metadata:
  labels:
    kagenti/mcp: "true"
```

### Tool prefixes

Each server uses a unique `toolPrefix` to avoid tool name conflicts:

| Server | Prefix | Example tool |
|--------|--------|--------------|
| moon | `moon_` | `moon_get_moon_phase` |
| quotes | `quotes_` | `quotes_get_quote` |
| weather | `weather_` | `weather_get_weather` |

## Hostnames

The HTTPRoutes use `.mcp.local` hostnames for internal routing:

- `moon.mcp.local`
- `quotes.mcp.local`
- `weather.mcp.local`

These hostnames are used by Envoy for internal routing and don't need DNS
resolution.
