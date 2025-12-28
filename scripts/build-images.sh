#!/bin/bash
# Build all MCP server container images with Podman
#
# Images are tagged for the local Kind registry at localhost:5000

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="${SCRIPT_DIR}/.."

REGISTRY="localhost:5000"
VERSION="v1.0.0"

echo "Building MCP server container images..."
echo "Registry: ${REGISTRY}"
echo "Version: ${VERSION}"
echo ""

cd "${PROJECT_DIR}"

# Build all servers
for SERVER in moon-server quotes-server weather-server; do
    echo "Building ${SERVER}..."
    podman build -t "${REGISTRY}/${SERVER}:${VERSION}" "${SERVER}/"
    echo "✓ ${SERVER} built successfully"
    echo ""
done

echo "All images built successfully!"
echo ""
echo "Images:"
podman images | grep -E "(REPOSITORY|localhost:5000)"
echo ""
echo "Next: Push images with ./scripts/push-images.sh"
