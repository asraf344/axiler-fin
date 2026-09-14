#!/bin/bash

set -e

REGISTRY="${1:-axiler-fin}"
VERSION="${2:-dev}"

echo "Building images for registry: $REGISTRY:$VERSION"
echo ""

SERVICES=("edge" "auth" "search" "transfer")
DIGESTS=()

for service in "${SERVICES[@]}"; do
    echo "Building $service service..."
    
    docker build \
        -t "$REGISTRY/$service:$VERSION" \
        -f "infra/docker/Dockerfile.$service" \
        .
    
    # Get digest
    DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' "$REGISTRY/$service:$VERSION" 2>/dev/null || echo "")
    echo "  → $DIGEST"
    DIGESTS+=("$DIGEST")
done

echo ""
echo "Build complete! Save these digests:"
for digest in "${DIGESTS[@]}"; do
    echo "  $digest"
done