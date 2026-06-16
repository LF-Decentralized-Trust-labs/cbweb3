#!/usr/bin/env bash
# setup-noc-agents.sh — Provisions the three NOC agent spokes and API keys
# in the Scenario B NOC backend (hub + spoke-a + spoke-b).
#
# The NOC backend exposes two admin endpoints (Bearer token required):
#   POST /api/v1/admin/spokes                 — create/register a spoke
#   POST /api/v1/admin/agents/provision-key   — bind a pre-shared key to a spoke
#
# Run AFTER compose.noc.yml is up and Keycloak is configured.
#
# Usage (from scenario-b/):
#   bash deploy/local/keycloak/setup-noc-agents.sh
#
# Environment variables (all optional):
#   NOC_BACKEND      Base URL of the NOC backend  (default: http://localhost:8091)
#   NOC_ADMIN_TOKEN  Bearer token for the NOC admin (obtained from Keycloak ROPC if empty)
#   KC_URL           Keycloak URL for ROPC token fetch (default: http://localhost:8081)
#   KC_REALM         Keycloak realm               (default: cbweb3)
#   KC_CLIENT_ID     Keycloak client ID           (default: noc-portal)
#   NOC_USER         NOC admin username           (default: noc.admin)
#   NOC_PASSWORD     NOC admin password           (default: NOCAdmin2026!)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NOC_BACKEND="${NOC_BACKEND:-http://localhost:8091}"
KC_URL="${KC_URL:-http://localhost:8081}"
KC_REALM="${KC_REALM:-cbweb3}"
KC_CLIENT_ID="${KC_CLIENT_ID:-noc-portal}"
NOC_USER="${NOC_USER:-noc.admin}"
NOC_PASSWORD="${NOC_PASSWORD:-NOCAdmin2026!}"

# Fixed UUIDs — must match agent.yaml configs mounted into the agent containers
HUB_SPOKE_ID="b0000000-0000-0000-0000-000000000001"
SPOKE_A_ID="b1000000-0000-0000-0000-000000000001"
SPOKE_B_ID="b2000000-0000-0000-0000-000000000001"

HUB_AGENT_KEY="noc-agent-hub-key-local"
SPOKE_A_AGENT_KEY="noc-agent-spoke-a-key-local"
SPOKE_B_AGENT_KEY="noc-agent-spoke-b-key-local"

echo -e "${YELLOW}=== Scenario B NOC Agent Setup ===${NC}"
echo -e "  NOC backend : ${NOC_BACKEND}"
echo ""

# ── Obtain bearer token via ROPC (unless pre-supplied) ───────────────────────
if [[ -z "${NOC_ADMIN_TOKEN:-}" ]]; then
  echo -e "${BLUE}Fetching auth token from Keycloak...${NC}"
  TOKEN_RESP=$(curl -s -X POST \
    "${KC_URL}/realms/${KC_REALM}/protocol/openid-connect/token" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=password" \
    -d "client_id=${KC_CLIENT_ID}" \
    -d "username=${NOC_USER}" \
    -d "password=${NOC_PASSWORD}")

  NOC_ADMIN_TOKEN=$(echo "${TOKEN_RESP}" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
  if [[ -z "${NOC_ADMIN_TOKEN}" ]]; then
    echo -e "${RED}ERROR: Could not obtain token. Response: ${TOKEN_RESP}${NC}"
    exit 1
  fi
  echo -e "${GREEN}  Token obtained.${NC}"
fi

AUTH_HEADER="Authorization: Bearer ${NOC_ADMIN_TOKEN}"

# ── Helper: create a spoke via POST /api/v1/admin/spokes ─────────────────────
create_spoke() {
  local id="$1"
  local name="$2"
  local currency="$3"
  local jurisdiction="$4"

  echo -e "${BLUE}Creating spoke '${name}' (${id})...${NC}"

  RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "${NOC_BACKEND}/api/v1/admin/spokes" \
    -H "${AUTH_HEADER}" \
    -H "Content-Type: application/json" \
    -d "{
      \"id\": \"${id}\",
      \"name\": \"${name}\",
      \"currency_code\": \"${currency}\",
      \"jurisdiction\": \"${jurisdiction}\"
    }")

  HTTP_STATUS=$(echo "${RESP}" | tail -1)
  BODY=$(echo "${RESP}" | sed '$d')

  if [[ "${HTTP_STATUS}" == "201" || "${HTTP_STATUS}" == "200" ]]; then
    echo -e "${GREEN}  Spoke '${name}' created (HTTP ${HTTP_STATUS}).${NC}"
  elif [[ "${HTTP_STATUS}" == "409" ]]; then
    echo -e "${YELLOW}  Spoke '${name}' already exists — skipping creation.${NC}"
  else
    echo -e "${RED}  ERROR creating spoke '${name}': HTTP ${HTTP_STATUS} — ${BODY}${NC}"
    exit 1
  fi
}

# ── Helper: provision an agent key via POST /api/v1/admin/agents/provision-key
provision_key() {
  local spoke_id="$1"
  local spoke_name="$2"
  local raw_key="$3"
  local hint="$4"

  echo -e "${BLUE}Provisioning agent key for '${spoke_name}'...${NC}"

  RESP=$(curl -s -w "\n%{http_code}" \
    -X POST "${NOC_BACKEND}/api/v1/admin/agents/provision-key" \
    -H "${AUTH_HEADER}" \
    -H "Content-Type: application/json" \
    -d "{
      \"raw_key\": \"${raw_key}\",
      \"spoke_id\": \"${spoke_id}\",
      \"hint\": \"${hint}\"
    }")

  HTTP_STATUS=$(echo "${RESP}" | tail -1)
  BODY=$(echo "${RESP}" | sed '$d')

  if [[ "${HTTP_STATUS}" == "201" || "${HTTP_STATUS}" == "200" ]]; then
    echo -e "${GREEN}  Key provisioned for '${spoke_name}' (HTTP ${HTTP_STATUS}).${NC}"
  elif [[ "${HTTP_STATUS}" == "409" ]]; then
    echo -e "${YELLOW}  Key for '${spoke_name}' already exists — skipping.${NC}"
  else
    echo -e "${RED}  ERROR provisioning key for '${spoke_name}': HTTP ${HTTP_STATUS} — ${BODY}${NC}"
    exit 1
  fi
}

# ── Register spokes ───────────────────────────────────────────────────────────
create_spoke "${HUB_SPOKE_ID}"  "Regional Hub" "HUB"  "SA-HUB"
create_spoke "${SPOKE_A_ID}"    "Spoke A"      "BRL"  "BR"
create_spoke "${SPOKE_B_ID}"    "Spoke B"      "ARS"  "AR"

echo ""

# ── Provision agent keys ──────────────────────────────────────────────────────
provision_key "${HUB_SPOKE_ID}"  "Regional Hub" "${HUB_AGENT_KEY}"     "hub-local-dev"
provision_key "${SPOKE_A_ID}"    "Spoke A"      "${SPOKE_A_AGENT_KEY}" "spoke-a-local-dev"
provision_key "${SPOKE_B_ID}"    "Spoke B"      "${SPOKE_B_AGENT_KEY}" "spoke-b-local-dev"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN} Scenario B NOC agents setup complete!${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""
echo -e "  Spokes registered:"
echo -e "    Hub     : ${HUB_SPOKE_ID}   key=${HUB_AGENT_KEY}"
echo -e "    Spoke A : ${SPOKE_A_ID}  key=${SPOKE_A_AGENT_KEY}"
echo -e "    Spoke B : ${SPOKE_B_ID}  key=${SPOKE_B_AGENT_KEY}"
echo ""
