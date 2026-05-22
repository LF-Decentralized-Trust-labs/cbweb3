#!/usr/bin/env bash
# setup-noc-realm.sh — Creates the NOC Portal realm, client, roles and default user
# in an already-running Keycloak instance (cbweb3-keycloak container).
#
# Usage:
#   ./deploy/local/keycloak/setup-noc-realm.sh
#   KC_CONTAINER=my-keycloak KC_ADMIN_PASS=secret ./deploy/local/keycloak/setup-noc-realm.sh
#
# Environment variables (all optional — defaults match local dev):
#   KC_CONTAINER      Keycloak container name  (default: cbweb3-keycloak)
#   KC_ADMIN_USER     Keycloak admin username   (default: admin)
#   KC_ADMIN_PASS     Keycloak admin password   (default: admin)
#   NOC_REALM         Realm name to create      (default: cbweb3)
#   NOC_CLIENT_ID     OIDC client ID            (default: noc-portal)
#   NOC_USER          Default NOC user login     (default: noc-admin)
#   NOC_PASSWORD      Default NOC user password  (default: noc-admin)

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

KC_CONTAINER="${KC_CONTAINER:-cbweb3-keycloak}"
KC_ADMIN_USER="${KC_ADMIN_USER:-admin}"
KC_ADMIN_PASS="${KC_ADMIN_PASS:-admin}"
NOC_REALM="${NOC_REALM:-cbweb3}"
NOC_CLIENT_ID="${NOC_CLIENT_ID:-noc-portal}"
NOC_USER="${NOC_USER:-noc-admin}"
NOC_PASSWORD="${NOC_PASSWORD:-noc-admin}"

KCADM="docker exec ${KC_CONTAINER} /opt/keycloak/bin/kcadm.sh"

echo -e "${YELLOW}=== NOC Keycloak Realm Setup ===${NC}"
echo -e "  Container : ${KC_CONTAINER}"
echo -e "  Realm     : ${NOC_REALM}"
echo -e "  Client    : ${NOC_CLIENT_ID}"
echo -e "  NOC user  : ${NOC_USER}"
echo ""

# ── Check container is running ────────────────────────────────────────────────
if ! docker inspect "${KC_CONTAINER}" >/dev/null 2>&1; then
  echo -e "${RED}ERROR: Container '${KC_CONTAINER}' not found.${NC}"
  echo "       Run 'make deploy.up-infra' first."
  exit 1
fi

# ── Authenticate ─────────────────────────────────────────────────────────────
echo -e "${BLUE}Authenticating with Keycloak admin...${NC}"
${KCADM} config credentials \
  --server http://localhost:8080 \
  --realm master \
  --user "${KC_ADMIN_USER}" \
  --password "${KC_ADMIN_PASS}"

# ── Create realm ──────────────────────────────────────────────────────────────
echo -e "${BLUE}Creating realm '${NOC_REALM}'...${NC}"
if ${KCADM} get "realms/${NOC_REALM}" > /dev/null 2>&1; then
  echo -e "${YELLOW}  Realm '${NOC_REALM}' already exists — skipping.${NC}"
else
  ${KCADM} create realms \
    -s "realm=${NOC_REALM}" \
    -s enabled=true \
    -s "displayName=CBweb3 NOC" \
    -s sslRequired=none \
    -s accessTokenLifespan=86400
  echo -e "${GREEN}  Realm '${NOC_REALM}' created.${NC}"
fi

# ── Create realm roles ────────────────────────────────────────────────────────
echo -e "${BLUE}Creating NOC roles...${NC}"
for role in noc-viewer noc-operator noc-admin; do
  if ${KCADM} get roles -r "${NOC_REALM}" --fields name \
      | grep -q "\"${role}\""; then
    echo -e "${YELLOW}  Role '${role}' already exists — skipping.${NC}"
  else
    ${KCADM} create roles -r "${NOC_REALM}" -s "name=${role}"
    echo -e "${GREEN}  Role '${role}' created.${NC}"
  fi
done

# ── Create client (public, ROPC enabled) ─────────────────────────────────────
echo -e "${BLUE}Creating client '${NOC_CLIENT_ID}'...${NC}"
CLIENT_EXISTS=$(${KCADM} get clients -r "${NOC_REALM}" --fields clientId \
  | grep "\"${NOC_CLIENT_ID}\"" || true)

if [[ -n "${CLIENT_EXISTS}" ]]; then
  echo -e "${YELLOW}  Client '${NOC_CLIENT_ID}' already exists — skipping.${NC}"
else
  ${KCADM} create clients -r "${NOC_REALM}" \
    -s "clientId=${NOC_CLIENT_ID}" \
    -s enabled=true \
    -s publicClient=true \
    -s directAccessGrantsEnabled=true \
    -s standardFlowEnabled=false \
    -s "redirectUris=[\"http://localhost:5173/*\",\"http://localhost:5174/*\",\"http://localhost:5175/*\",\"http://localhost:5176/*\",\"http://localhost:5177/*\",\"http://localhost:5178/*\",\"http://localhost:5180/*\"]" \
    -s "webOrigins=[\"http://localhost:5173\",\"http://localhost:5174\",\"http://localhost:5175\",\"http://localhost:5176\",\"http://localhost:5177\",\"http://localhost:5178\",\"http://localhost:5180\"]"
  echo -e "${GREEN}  Client '${NOC_CLIENT_ID}' created (public, ROPC enabled).${NC}"
fi

# ── Create default NOC user ───────────────────────────────────────────────────
echo -e "${BLUE}Creating user '${NOC_USER}'...${NC}"
USER_EXISTS=$(${KCADM} get users -r "${NOC_REALM}" -q "username=${NOC_USER}" | grep "\"${NOC_USER}\"" || true)

if [[ -n "${USER_EXISTS}" ]]; then
  echo -e "${YELLOW}  User '${NOC_USER}' already exists — skipping creation.${NC}"
else
  ${KCADM} create users -r "${NOC_REALM}" \
    -s "username=${NOC_USER}" \
    -s enabled=true \
    -s "email=${NOC_USER}@local.dev" \
    -s emailVerified=true \
    -s "firstName=NOC" \
    -s "lastName=Admin"
  ${KCADM} set-password -r "${NOC_REALM}" \
    --username "${NOC_USER}" \
    --new-password "${NOC_PASSWORD}"
  echo -e "${GREEN}  User '${NOC_USER}' created.${NC}"
fi

# ── Assign noc-admin role to user ─────────────────────────────────────────────
echo -e "${BLUE}Assigning 'noc-admin' role to '${NOC_USER}'...${NC}"
${KCADM} add-roles \
  -r "${NOC_REALM}" \
  --uusername "${NOC_USER}" \
  --rolename noc-admin || true
echo -e "${GREEN}  Role assigned.${NC}"

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN} NOC Keycloak realm setup complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "  Realm    : ${NOC_REALM}"
echo -e "  Client   : ${NOC_CLIENT_ID}"
echo -e "  Roles    : noc-viewer  noc-operator  noc-admin"
echo -e "  Username : ${NOC_USER}"
echo -e "  Password : ${NOC_PASSWORD}"
echo ""
echo -e "  Frontend .env.local:"
echo -e "    VITE_KEYCLOAK_URL=http://localhost:8081"
echo -e "    VITE_KEYCLOAK_REALM=${NOC_REALM}"
echo -e "    VITE_KEYCLOAK_CLIENT_ID=${NOC_CLIENT_ID}"
echo ""
