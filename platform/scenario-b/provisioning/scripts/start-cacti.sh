#!/usr/bin/env bash
# start-cacti.sh — bring up the Cacti liquidity relay locally (Scenario B).
#
# The relay is deployed EXTERNALLY to the toolkit (scenario-a strategy): its
# address is then passed to the toolkit via each manifest's spec.relay.endpoint.
# The relay boots NEUTRAL (no fixed spokes) and exposes the TK-B5 spoke registry,
# so a founding central bank registers its spoke at found-spoke via
# POST /api/v1/spokes (register-relay-spoke), confirmed by GET /api/v1/spokes.
#
# Usage:
#   provisioning/scripts/start-cacti.sh            # up (build) + wait for health
#   CACTI_PORT=7000 provisioning/scripts/start-cacti.sh
#   provisioning/scripts/start-cacti.sh --down     # tear the relay down
#
# Environment overrides:
#   CACTI_PORT      Host port for the relay REST API (default: 7000)
#   CACTI_COMPOSE   Path to the relay docker-compose.yaml (default: repo path)
#   TIMEOUT_SECS    Max seconds to wait for the health endpoint (default: 120)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# provisioning/scripts → scenario-b
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CACTI_COMPOSE="${CACTI_COMPOSE:-${SCENARIO_DIR}/interop/hub-and-spoke/cacti/docker-compose.yaml}"
CACTI_PORT="${CACTI_PORT:-7000}"
CACTI_URL="http://localhost:${CACTI_PORT}"
TIMEOUT_SECS="${TIMEOUT_SECS:-120}"

log() { echo "[start-cacti] $*"; }

if [ ! -f "${CACTI_COMPOSE}" ]; then
  log "ERROR: compose file not found at ${CACTI_COMPOSE}"
  exit 1
fi

# Teardown mode.
if [ "${1:-}" = "--down" ]; then
  log "Stopping Cacti relay…"
  CACTI_API_PORT="${CACTI_PORT}" docker compose -f "${CACTI_COMPOSE}" down
  log "Done."
  exit 0
fi

log "Starting Cacti liquidity relay from ${CACTI_COMPOSE}"
CACTI_API_PORT="${CACTI_PORT}" docker compose -f "${CACTI_COMPOSE}" up -d --build

log "Waiting for relay health (${CACTI_URL}/api/v1/health, max ${TIMEOUT_SECS}s)…"
elapsed=0
healthy=false
while [ "${elapsed}" -lt "${TIMEOUT_SECS}" ]; do
  if curl -sf "${CACTI_URL}/api/v1/health" >/dev/null 2>&1; then
    healthy=true
    break
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

if [ "${healthy}" != "true" ]; then
  log "ERROR: relay did not become healthy within ${TIMEOUT_SECS}s"
  log "Check logs with: docker compose -f ${CACTI_COMPOSE} logs"
  exit 1
fi

log "Relay healthy at ${CACTI_URL}"
echo "  Register spoke : POST ${CACTI_URL}/api/v1/spokes   (used by found-spoke register-relay-spoke)"
echo "  List spokes    : GET  ${CACTI_URL}/api/v1/spokes"
echo "  Point manifests' spec.relay.endpoint at: ${CACTI_URL}"
