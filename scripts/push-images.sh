#!/bin/bash
# Push all MCP server container images to local Kind registry
#
# Prerequisites: Run setup-kind-with-registry.sh first to create the registry

set -e

REGISTRY="localhost:5000"
VERSION="v1.0.0"

echo "Pushing MCP server container images to registry..."
echo "Registry: $REGISTRY"
echo "Version: $VERSION"
echo ""

# Check if registry is accessible
echo "Checking registry connectivity..."
if ! curl -s -f "http://${REGISTRY}/v2/" > /dev/null 2>&1; then
    echo "Error: Registry at ${REGISTRY} is not accessible"
    echo ""
    echo "Make sure you've run: ./scripts/setup-kind-with-registry.sh"
    echo "Or check that the kind-registry container is running:"
    echo "  podman ps | grep kind-registry"
    exit 1
fi
echo "✓ Registry is accessible"
echo ""

# Push images
for SERVER in moon-server quotes-server weather-server; do
    echo "Pushing ${SERVER}..."
    podman push "${REGISTRY}/${SERVER}:${VERSION}" --tls-verify=false
    echo "✓ ${SERVER} pushed successfully"
    echo ""
done

echo "All images pushed successfully!"
echo ""
echo "Verify images in registry:"
curl -s "http://${REGISTRY}/v2/_catalog" | jq . 2>/dev/null || curl -s "http://${REGISTRY}/v2/_catalog"
echo ""
echo "Next: Deploy with ./scripts/deploy.sh"
