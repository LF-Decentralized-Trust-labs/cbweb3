#!/usr/bin/env bash
# Build the GENERIC launcher image (no per-entity URLs baked in). This is a
# platform-level step, run once and independent of any scenario — the scenario
# toolkits only `docker run` this image and mount each entity's config fragments.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="${IMAGE:-cbweb3/launcher:local}"

echo "Building launcher image: $IMAGE"
docker build -t "$IMAGE" "$SCRIPT_DIR"
echo "✅ built $IMAGE"
