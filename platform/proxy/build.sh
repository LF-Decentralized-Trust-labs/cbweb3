#!/usr/bin/env bash
# Build the GENERIC reverse-proxy image (no per-entity hosts/ports baked in). This is
# a platform-level step, run once and independent of any scenario — the scenario
# toolkits only `docker run` this image and mount each entity's route fragments.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="${IMAGE:-cbweb3/proxy:local}"

echo "Building proxy image: $IMAGE"
docker build -t "$IMAGE" "$SCRIPT_DIR"
echo "✅ built $IMAGE"
