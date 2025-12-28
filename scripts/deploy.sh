#!/bin/bash
# Deploy all MCP servers to Kubernetes cluster
#
# Prerequisites:
#   - Run setup-kind-with-registry.sh to create cluster with registry
#   - Run build-images.sh to build images
#   - Run push-images.sh to push images to registry

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="${SCRIPT_DIR}/../k8s"

echo "Deploying MCP servers to Kubernetes cluster..."
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed or not in PATH"
    exit 1
fi

# Check cluster connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster"
    echo "Make sure you've run: ./scripts/setup-kind-with-registry.sh"
    exit 1
fi

# Deploy moon-server
echo "=== Deploying moon-server ==="
kubectl apply -f "${K8S_DIR}/moon-server/namespace.yaml"
kubectl apply -f "${K8S_DIR}/moon-server/deployment.yaml"
kubectl apply -f "${K8S_DIR}/moon-server/service.yaml"
echo "✓ moon-server manifests applied"
echo ""

# Deploy quotes-server
echo "=== Deploying quotes-server ==="
kubectl apply -f "${K8S_DIR}/quotes-server/namespace.yaml"
kubectl apply -f "${K8S_DIR}/quotes-server/deployment.yaml"
kubectl apply -f "${K8S_DIR}/quotes-server/service.yaml"
echo "✓ quotes-server manifests applied"
echo ""

# Deploy weather-server
echo "=== Deploying weather-server ==="
kubectl apply -f "${K8S_DIR}/weather-server/namespace.yaml"
kubectl apply -f "${K8S_DIR}/weather-server/deployment.yaml"
kubectl apply -f "${K8S_DIR}/weather-server/service.yaml"
echo "✓ weather-server manifests applied"
echo ""

# Wait for all deployments
echo "=== Waiting for deployments ==="
kubectl wait --for=condition=available --timeout=120s deployment/moon-server -n moon-server &
kubectl wait --for=condition=available --timeout=120s deployment/quotes-server -n quotes-server &
kubectl wait --for=condition=available --timeout=120s deployment/weather-server -n weather-server &
wait
echo "✓ All deployments ready"
echo ""

echo "=== Deployment Summary ==="
echo ""
echo "All services deployed successfully!"
echo ""
kubectl get pods -A | grep -E '(NAMESPACE|moon-server|quotes-server|weather-server)'
echo ""
echo "Service endpoints (from within cluster):"
echo "  - moon: http://moon-service.moon-server.svc.cluster.local:8081/mcp"
echo "  - quotes: http://quotes-service.quotes-server.svc.cluster.local:8082/mcp"
echo "  - weather: http://weather-service.weather-server.svc.cluster.local:8083/mcp"
echo ""
echo "To test from host, use port-forward:"
echo "  kubectl port-forward -n moon-server svc/moon-service 8081:8081"
echo "  curl http://localhost:8081/health"
