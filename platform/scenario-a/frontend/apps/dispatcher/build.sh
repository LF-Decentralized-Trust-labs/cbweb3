#!/bin/bash
# Build script for Dispatcher Portal Docker image
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
PLATFORM="${PLATFORM:-linux/amd64}"
IMAGE_TAG="${IMAGE_TAG:-cbweb3/dispatcher}"
PUSH="${PUSH:-false}"

echo "Building Dispatcher Portal Docker image..."
echo "Frontend dir: $FRONTEND_DIR"
echo "Dockerfile: $SCRIPT_DIR/Dockerfile"
echo "Platform: $PLATFORM"
echo "Image tag: $IMAGE_TAG"

# Ensure buildx builder exists
if ! docker buildx inspect multiplatform &>/dev/null; then
  echo "Creating multiplatform builder..."
  docker buildx create --name multiplatform --driver docker-container --use
  docker buildx inspect multiplatform --bootstrap
fi

# Build arguments
BUILD_ARGS=(
  --platform "$PLATFORM"
  --file "$SCRIPT_DIR/Dockerfile"
  --tag "$IMAGE_TAG"
  --build-arg "VITE_BANK_A_PORTAL_URL=${VITE_BANK_A_PORTAL_URL:-http://15.235.15.105:5173}"
  --build-arg "VITE_BANK_B_PORTAL_URL=${VITE_BANK_B_PORTAL_URL:-http://15.235.15.105:5174}"
  --build-arg "VITE_BANK_C_PORTAL_URL=${VITE_BANK_C_PORTAL_URL:-http://15.235.15.105:5175}"
  --build-arg "VITE_BANK_D_PORTAL_URL=${VITE_BANK_D_PORTAL_URL:-http://15.235.15.105:5176}"
  --build-arg "VITE_CENTRAL_BANK_A_PORTAL_URL=${VITE_CENTRAL_BANK_A_PORTAL_URL:-http://15.235.15.105:5177}"
  --build-arg "VITE_CENTRAL_BANK_B_PORTAL_URL=${VITE_CENTRAL_BANK_B_PORTAL_URL:-http://15.235.15.105:5178}"
)

# Add --push or --load depending on mode
if [ "$PUSH" = "true" ]; then
  BUILD_ARGS+=(--push)
  echo "Mode: Push to registry"
else
  BUILD_ARGS+=(--load)
  echo "Mode: Load to local Docker"
fi

# Build from frontend directory (context) but use the dispatcher Dockerfile
docker buildx build "${BUILD_ARGS[@]}" "$FRONTEND_DIR"

echo ""
echo "✅ Build complete!"
echo "Image: $IMAGE_TAG"
echo "Platform: $PLATFORM"
if [ "$PUSH" != "true" ]; then
  echo ""
  echo "To run:"
  echo "  docker run -p 5180:80 $IMAGE_TAG"
fi
