#!/bin/bash
# Setup Kind cluster with a local registry for Podman on macOS
#
# This script creates:
# 1. A local container registry (running as a Podman container, NOT in k8s)
# 2. A Kind cluster connected to the same network
# 3. Proper configuration so the cluster can pull from the registry
#
# After setup:
# - Push images as: localhost:5000/image:tag (from host)
# - Pods pull as: kind-registry:5000/image:tag (inside cluster)
#
# Based on: https://kind.sigs.k8s.io/docs/user/local-registry/

set -o errexit

# Configuration
CLUSTER_NAME="${KIND_CLUSTER_NAME:-mcp-cluster}"
REGISTRY_NAME="kind-registry"
REGISTRY_PORT="${REGISTRY_PORT:-5000}"

echo "=== Setting up Kind cluster with local registry ==="
echo "Cluster name: ${CLUSTER_NAME}"
echo "Registry: ${REGISTRY_NAME}:${REGISTRY_PORT}"
echo ""

# 1. Create registry container if it doesn't exist
if [ "$(podman inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" 2>/dev/null || true)" != 'true' ]; then
    echo "Creating registry container..."

    # Remove existing stopped container if any
    podman rm -f "${REGISTRY_NAME}" 2>/dev/null || true

    # Use a named volume for persistence (works better with Podman on macOS)
    podman volume create kind-registry-data 2>/dev/null || true

    podman run \
        -d \
        --restart=always \
        --name "${REGISTRY_NAME}" \
        -p "127.0.0.1:${REGISTRY_PORT}:5000" \
        -v kind-registry-data:/var/lib/registry \
        docker.io/library/registry:2

    echo "✓ Registry container created"
else
    echo "✓ Registry container already running"
fi

# 2. Create Kind cluster with containerd registry config
echo ""
echo "Creating Kind cluster..."

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CONFIG="${SCRIPT_DIR}/../kind-config.yaml"

if [ ! -f "${KIND_CONFIG}" ]; then
    echo "Error: kind-config.yaml not found at ${KIND_CONFIG}"
    exit 1
fi

KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name "${CLUSTER_NAME}" --config="${KIND_CONFIG}"

# Export kubeconfig to ensure kubectl uses the new cluster
KIND_EXPERIMENTAL_PROVIDER=podman kind export kubeconfig --name "${CLUSTER_NAME}"

echo "✓ Kind cluster created"

# 3. Connect registry to Kind network (if not already connected)
echo ""
echo "Connecting registry to Kind network..."

# Get the Kind network name (usually "kind" or "podman")
KIND_NETWORK="kind"

# Connect the registry to the network
if ! podman network inspect "${KIND_NETWORK}" | grep -q "${REGISTRY_NAME}"; then
    podman network connect "${KIND_NETWORK}" "${REGISTRY_NAME}" 2>/dev/null || true
    echo "✓ Registry connected to Kind network"
else
    echo "✓ Registry already connected to Kind network"
fi

# 4. Configure containerd on Kind nodes to use the registry
echo ""
echo "Configuring containerd to use local registry..."

# Get registry IP on the kind network
REGISTRY_IP=$(podman inspect -f '{{.NetworkSettings.Networks.kind.IPAddress}}' "${REGISTRY_NAME}" 2>/dev/null || echo "")

# Fallback: use the registry name as hostname (should resolve via Podman DNS)
if [ -z "${REGISTRY_IP}" ]; then
    REGISTRY_IP="${REGISTRY_NAME}"
fi

echo "Registry accessible at: ${REGISTRY_IP}:5000 (inside Kind network)"

# Create the registry hosts configuration on the Kind node
for NODE in $(kind get nodes --name "${CLUSTER_NAME}"); do
    echo "Configuring node: ${NODE}"

    # Create directory for registry config (using kind-registry hostname)
    podman exec "${NODE}" mkdir -p "/etc/containerd/certs.d/${REGISTRY_NAME}:5000"

    # Write hosts.toml for the registry
    # This tells containerd to use HTTP (not HTTPS) for kind-registry:5000
    HOSTS_TOML="server = \"http://${REGISTRY_NAME}:5000\"

[host.\"http://${REGISTRY_NAME}:5000\"]
  capabilities = [\"pull\", \"resolve\", \"push\"]
"
    podman exec "${NODE}" bash -c "echo '${HOSTS_TOML}' > /etc/containerd/certs.d/${REGISTRY_NAME}:5000/hosts.toml"

    # Restart containerd to pick up the new configuration
    echo "Restarting containerd on ${NODE}..."
    podman exec "${NODE}" systemctl restart containerd

    echo "✓ Configured ${NODE}"
done

# Wait for node to be ready after containerd restart
echo ""
echo "Waiting for node to be ready..."
kubectl wait --for=condition=Ready node --all --timeout=60s

# 5. Document the registry for Kind
# This creates a ConfigMap that tells tools like Tilt about the registry
echo ""
echo "Creating registry ConfigMap..."

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

echo "✓ Registry ConfigMap created"

# 6. Verify setup
echo ""
echo "=== Verifying setup ==="

echo "Checking registry connectivity from host..."
if curl -s -f "http://localhost:${REGISTRY_PORT}/v2/" > /dev/null 2>&1; then
    echo "✓ Registry accessible from host at localhost:${REGISTRY_PORT}"
else
    echo "⚠ Registry not accessible from host (may take a moment to start)"
fi

echo ""
echo "Checking cluster status..."
kubectl cluster-info --context "kind-${CLUSTER_NAME}"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Usage:"
echo "  1. Build images with localhost:${REGISTRY_PORT} tag:"
echo "     podman build -t localhost:${REGISTRY_PORT}/my-image:v1 ."
echo ""
echo "  2. Push to registry (from host):"
echo "     podman push localhost:${REGISTRY_PORT}/my-image:v1 --tls-verify=false"
echo ""
echo "  3. Use in Kubernetes (use registry container name):"
echo "     image: ${REGISTRY_NAME}:5000/my-image:v1"
echo ""
echo "  Note: Push as localhost:5000, pull as kind-registry:5000"
echo "        (same registry, different hostnames for host vs cluster)"
echo ""
echo "  4. To delete cluster and registry:"
echo "     kind delete cluster --name ${CLUSTER_NAME}"
echo "     podman rm -f ${REGISTRY_NAME}"
echo ""
