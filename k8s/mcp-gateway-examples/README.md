# MCP Gateway example manifests

These manifests configure the sample MCP servers to work with
[mcp-gateway](https://github.com/Kuadrant/mcp-gateway).

See also:

- [NAMESPACE-STRATEGIES.md](NAMESPACE-STRATEGIES.md) - Guide to organizing
  resources across namespaces

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

## Troubleshooting

### MCPServer shows "0 servers with 0 tools"

If your MCPServer status shows:

```text
MCPServer successfully reconciled and validated 0 servers with 0 tools
```

This indicates the controller found the MCPServer but the broker hasn't
registered any tools. Follow these debugging steps:

#### Step 1: Check HTTPRoute label

Verify the HTTPRoute has the required label:

```bash
kubectl get httproute <route-name> -n mcp-test -o yaml | grep -A2 "labels:"
```

Expected output:

```yaml
labels:
  mcp-server: "true"
```

If missing, add it:

```bash
kubectl label httproute <route-name> -n mcp-test mcp-server=true
```

#### Step 2: Test MCP server connectivity

Verify the backend MCP server is reachable:

```bash
# Test health endpoint
kubectl run curl-test --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s http://<service>.<namespace>.svc.cluster.local:<port>/health

# Test MCP initialize
kubectl run curl-test --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  http://<service>.<namespace>.svc.cluster.local:<port>/mcp
```

#### Step 3: Check controller logs

```bash
kubectl logs -n mcp-system -l app.kubernetes.io/name=mcp-gateway | grep -i "moon\|error\|fail"
```

Look for:

- "Successfully regenerated aggregated configuration serverCount: 1" (good)
- "No MCPServers found" (bad - check labels)
- "NOT validating" with mismatched IDs (see ConfigMap issue below)

#### Step 4: Check broker status

```bash
# Get broker pod IP
BROKER_IP=$(kubectl get pod -n mcp-system -l app.kubernetes.io/name=mcp-gateway \
  -o jsonpath='{.items[?(@.metadata.name contains "broker")].status.podIP}')

# Check status endpoint
kubectl run curl-test --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s http://${BROKER_IP}:8080/status
```

Expected: `"totalServers": 1, "healthyServers": 1, "toolCount": 2`

### Known issue: Controller not updating ConfigMap

There is a known issue where the controller logs "Successfully regenerated
aggregated configuration" but never actually writes to the ConfigMap. You can
verify this by checking the ConfigMap resourceVersion - if it never changes,
the controller isn't writing.

```bash
# Check if ConfigMap is being updated
kubectl get configmap mcp-gateway-config -n mcp-system -o jsonpath='{.metadata.resourceVersion}'
```

#### Workaround: Manual ConfigMap patch

If the controller isn't updating the ConfigMap, you can manually patch it.

**Important**: The server name must use the format `<namespace>/<httproute-name>`:

```bash
kubectl patch configmap mcp-gateway-config -n mcp-system --type merge -p '
{
  "data": {
    "config.yaml": "servers:\n  - name: mcp-test/moon-route\n    url: http://moon-service.moon-server.svc.cluster.local:8081/mcp\n    hostname: moon.mcp.local\n    enabled: true\n    toolPrefix: \"moon_\"\nvirtualServers: []\n"
  }
}'
```

For multiple servers:

```bash
kubectl patch configmap mcp-gateway-config -n mcp-system --type merge -p '
{
  "data": {
    "config.yaml": "servers:\n  - name: mcp-test/moon-route\n    url: http://moon-service.moon-server.svc.cluster.local:8081/mcp\n    hostname: moon.mcp.local\n    enabled: true\n    toolPrefix: \"moon_\"\n  - name: mcp-test/quotes-route\n    url: http://quotes-service.quotes-server.svc.cluster.local:8082/mcp\n    hostname: quotes.mcp.local\n    enabled: true\n    toolPrefix: \"quotes_\"\n  - name: mcp-test/weather-route\n    url: http://weather-service.weather-server.svc.cluster.local:8083/mcp\n    hostname: weather.mcp.local\n    enabled: true\n    toolPrefix: \"weather_\"\nvirtualServers: []\n"
  }
}'
```

After patching, restart the broker to pick up the new config:

```bash
kubectl rollout restart deployment mcp-gateway-broker-router -n mcp-system
```

Then restart the controller to refresh status:

```bash
kubectl rollout restart deployment mcp-gateway-controller -n mcp-system
```

Verify the MCPServer status:

```bash
kubectl get mcpserver -n mcp-test -o jsonpath='{.items[0].status.conditions[0].message}'
# Expected: MCPServer successfully reconciled and validated 1 servers with 2 tools
```

### Server ID format mismatch

The controller expects server IDs in the format:

```text
<namespace>/<httproute-name>:<toolPrefix>:<url>
```

For example:

```text
mcp-test/moon-route:moon_:http://moon-service.moon-server.svc.cluster.local:8081/mcp
```

If the ConfigMap uses a different format (like just the MCPServer name), the
controller will log "NOT validating" with mismatched IDs and report 0 tools.

### Wrong gateway namespace in tool calls (gateway-system error)

When calling tools, you may see an error like:

```text
Post "http://mcp-gateway-istio.gateway-system.svc.cluster.local:8080/mcp":
dial tcp: lookup mcp-gateway-istio.gateway-system.svc.cluster.local: no such host
```

**Cause:** The broker's `--mcp-gateway-private-host` flag has a wrong default
value that uses `gateway-system` namespace instead of `mcp-system`.

**Symptoms:**

- `initialize` and `tools/list` work fine
- `tools/call` fails with the above error
- Broker logs show: `failed to get remote session`

**Fix with kubectl patch:**

The Helm chart (as of v0.4.1) doesn't expose the `--mcp-gateway-private-host`
flag in values.yaml, so you must patch the deployment:

```bash
kubectl patch deployment mcp-gateway-broker-router -n mcp-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/command/-", "value": "--mcp-gateway-private-host=mcp-gateway-istio.mcp-system.svc.cluster.local:80"}]'
```

**Important:** This patch won't persist across `helm upgrade`. You'll need to
reapply it after each upgrade until the chart is fixed.

**Verify the fix:**

```bash
# Check the flag is set
kubectl get deployment mcp-gateway-broker-router -n mcp-system \
  -o jsonpath='{.spec.template.spec.containers[0].command}' | tr ',' '\n' | grep private-host

# Expected output:
# "--mcp-gateway-private-host=mcp-gateway-istio.mcp-system.svc.cluster.local:80"
```

### Reporting issues

If you encounter issues, please report them at:
<https://github.com/Kuadrant/mcp-gateway/issues>

For the ConfigMap not updating issue, include:

1. Controller logs showing "Successfully regenerated aggregated configuration"
1. ConfigMap resourceVersion staying unchanged
1. Your MCPServer and HTTPRoute manifests

For the gateway-system namespace issue, include:

1. Broker logs showing `failed to get remote session`
1. The Helm values you used during installation
