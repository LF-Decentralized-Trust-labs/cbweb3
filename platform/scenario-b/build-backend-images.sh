#!/bin/bash
# build-backend-images.sh
# Build all Docker images for cbweb3-platform backend services
# Usage: ./build-backend-images.sh

set -e

# Base path to Dockerfiles
BASE_DIR="$(dirname "$0")/backend/services"

# Backend services present in all docker-compose files
SERVICES=(
  "compliance"
  "auth"
  "payment-orchestrator"
  "api-gateway"
)


# TAG and VERSION via arguments or environment variables
TAG="${1:-${TAG:-local}}"
VERSION="${2:-${VERSION:-latest}}"

echo "\nUsing TAG: $TAG  VERSION: $VERSION"

# Parallel build
PIDS=()
for SERVICE in "${SERVICES[@]}"; do
  echo "\n==> Building image for service: $SERVICE"
  docker build -t cbweb3/$SERVICE:$TAG-$VERSION -f "$BASE_DIR/$SERVICE/Dockerfile" "$BASE_DIR/.." &
  PIDS+=("$!")
done

# Wait for all builds to finish
FAIL=0
for PID in "${PIDS[@]}"; do
  wait $PID || FAIL=1
done

if [ "$FAIL" -eq 0 ]; then
  echo "\nAll backend images built successfully."
else
  echo "\nOne or more builds failed. Check the logs above."
  exit 1
fi
