#!/usr/bin/env bash
# start-cacti.sh — Bring up the Cacti HTLC relay locally (Scenario A).
#
# Use this when LNET's centrally-operated relay is not available and you need a
# local relay for the toolkit's register-relay step (mode:found) to succeed.
# The relay exposes the RL-1 spoke-registry endpoints, so a founding central
# bank can register its spoke via POST /api/v1/spokes and the toolkit confirms
# it via GET /api/v1/spokes/:id.
#
# Usage:
#   provisioning/scripts/start-cacti.sh            # up + wait for health
#   CACTI_PORT=4000 provisioning/scripts/start-cacti.sh
#   provisioning/scripts/start-cacti.sh --down     # tear the relay down
#
# Environment overrides:
#   CACTI_PORT      Host port for the relay REST API (default: 4000)
#   CACTI_COMPOSE   Path to the relay docker-compose.yaml (default: repo path)
#   TIMEOUT_SECS    Max seconds to wait for the health endpoint (default: 90)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# provisioning/scripts → scenario-a
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CACTI_COMPOSE="${CACTI_COMPOSE:-${SCENARIO_DIR}/interop/hub-and-spoke/cacti/docker-compose.yaml}"
CACTI_PORT="${CACTI_PORT:-4000}"
CACTI_URL="http://localhost:${CACTI_PORT}"
TIMEOUT_SECS="${TIMEOUT_SECS:-90}"

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

log "Starting Cacti HTLC relay from ${CACTI_COMPOSE}"
CACTI_API_PORT="${CACTI_PORT}" docker compose -f "${CACTI_COMPOSE}" up -d --build

log "Waiting for relay health endpoint (${CACTI_URL}/api/v1/health, max ${TIMEOUT_SECS}s)…"
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

log "Relay is healthy at ${CACTI_URL}"
echo
echo "  Health         : GET  ${CACTI_URL}/api/v1/health"
echo "  Register spoke : POST ${CACTI_URL}/api/v1/spokes      (used by mode:found register-relay)"
echo "  List spokes    : GET  ${CACTI_URL}/api/v1/spokes"
echo "  Spoke status   : GET  ${CACTI_URL}/api/v1/spokes/<id>"
echo
echo "Point your manifests' spec.relay.endpoint at: ${CACTI_URL}"
