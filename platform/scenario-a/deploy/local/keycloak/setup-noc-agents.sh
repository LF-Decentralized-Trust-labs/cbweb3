#!/usr/bin/env bash
# setup-noc-agents.sh — Provisions NOC spokes and agent API keys in noc-backend.
#
# Requires noc-backend to be running and the cbweb3 Keycloak realm to exist
# (run setup-noc-realm.sh first).
#
# Environment variables (all optional — defaults match local dev):
#   NOC_BACKEND   noc-backend base URL    (default: http://localhost:8090)
#   KC_URL        Keycloak base URL       (default: http://localhost:8081)
#   KC_REALM      Keycloak realm          (default: cbweb3)
#   KC_CLIENT_ID  OIDC client             (default: noc-portal)
#   NOC_USER      NOC admin username      (default: noc-admin)
#   NOC_PASSWORD  NOC admin password      (default: noc-admin)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NOC_BACKEND="${NOC_BACKEND:-http://localhost:8090}"
KC_URL="${KC_URL:-http://localhost:8081}"
KC_REALM="${KC_REALM:-cbweb3}"
KC_CLIENT_ID="${KC_CLIENT_ID:-noc-portal}"
NOC_USER="${NOC_USER:-noc-admin}"
NOC_PASSWORD="${NOC_PASSWORD:-noc-admin}"

echo -e "${YELLOW}=== NOC Agent Provisioning ===${NC}"
echo -e "  Backend : ${NOC_BACKEND}"
echo -e "  Keycloak: ${KC_URL}/realms/${KC_REALM}"
echo ""

# ── Wait for NOC backend ──────────────────────────────────────────────────────
echo -e "${BLUE}Waiting for NOC backend to be ready...${NC}"
attempt=1
until curl -fsS "${NOC_BACKEND}/health" >/dev/null 2>&1; do
  if [ "$attempt" -ge 40 ]; then
    echo -e "${RED}ERROR: NOC backend did not become ready at ${NOC_BACKEND}/health${NC}"
    exit 1
  fi
  echo "  [${attempt}/40] waiting..."
  attempt=$((attempt + 1))
  sleep 3
done
echo -e "${GREEN}  NOC backend is ready.${NC}"

# ── Obtain Keycloak JWT ───────────────────────────────────────────────────────
echo -e "${BLUE}Obtaining Keycloak token for '${NOC_USER}'...${NC}"
TOKEN=$(curl -fsS -X POST \
  "${KC_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=${KC_CLIENT_ID}&username=${NOC_USER}&password=${NOC_PASSWORD}" \
  | jq -r '.access_token')

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo -e "${RED}ERROR: Failed to obtain Keycloak token. Is the cbweb3 realm set up?${NC}"
  exit 1
fi
echo -e "${GREEN}  Token obtained.${NC}"

# ── Helpers ───────────────────────────────────────────────────────────────────

# ensure_spoke <uuid> <name> <currency_code> <jurisdiction>
ensure_spoke() {
  local id="$1" name="$2" currency="$3" jurisdiction="$4"

  local http_status
  http_status=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${TOKEN}" \
    "${NOC_BACKEND}/api/v1/admin/spokes/${id}")

  if [[ "$http_status" == "200" ]]; then
    echo -e "${YELLOW}  Spoke '${name}' (${id}) already exists — skipping.${NC}"
    return 0
  fi

  local response
  response=$(curl -fsS -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    "${NOC_BACKEND}/api/v1/admin/spokes" \
    -d "{\"id\":\"${id}\",\"name\":\"${name}\",\"currency_code\":\"${currency}\",\"jurisdiction\":\"${jurisdiction}\"}")
  echo -e "${GREEN}  Spoke '${name}' created: $(echo "$response" | jq -r '.id')${NC}"
}

# provision_key <spoke_uuid> <raw_key> <hint>
provision_key() {
  local spoke_id="$1" raw_key="$2" hint="$3"

  local response
  response=$(curl -fsS -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    "${NOC_BACKEND}/api/v1/admin/agents/provision-key" \
    -d "{\"spoke_id\":\"${spoke_id}\",\"raw_key\":\"${raw_key}\",\"hint\":\"${hint}\"}")
  echo -e "${GREEN}  Key '${hint}' provisioned (prefix: $(echo "$response" | jq -r '.key_prefix')).${NC}"
}

# ── Provision spokes ──────────────────────────────────────────────────────────
echo -e "${BLUE}Provisioning spokes...${NC}"

ensure_spoke \
  "4bdea728-44b7-4434-a8df-329c8ebf7af7" \
  "spoke-a" "BRL" "Brazil"

ensure_spoke \
  "4c90f9e7-ab97-4360-86f2-5d5f485afb4e" \
  "spoke-b" "ARS" "Argentina"

# ── Provision agent API keys ──────────────────────────────────────────────────
echo -e "${BLUE}Provisioning agent API keys...${NC}"

provision_key \
  "4bdea728-44b7-4434-a8df-329c8ebf7af7" \
  "noc-agent-spoke-a-key-local" \
  "spoke-a-local-dev"

provision_key \
  "4c90f9e7-ab97-4360-86f2-5d5f485afb4e" \
  "noc-agent-spoke-b-key-local" \
  "spoke-b-local-dev"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} NOC agent provisioning complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "  Spoke A : 4bdea728-44b7-4434-a8df-329c8ebf7af7  (BRL / Brazil)"
echo -e "  Spoke B : 4c90f9e7-ab97-4360-86f2-5d5f485afb4e  (ARS / Argentina)"
echo -e "  Agents will begin pushing telemetry within one push_interval (~15s)."
echo ""
