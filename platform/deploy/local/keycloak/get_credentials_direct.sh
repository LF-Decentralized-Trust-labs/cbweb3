#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$(cd "$SCRIPT_DIR/.." && pwd)/.env.keycloak"
LOCAL_ENV_FILE="$(cd "$SCRIPT_DIR/.." && pwd)/.env"

if [[ -f "$LOCAL_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$LOCAL_ENV_FILE"
  set +a
fi

KEYCLOAK_CONTAINER_NAME="${KEYCLOAK_CONTAINER_NAME:-keycloak}"
KC_ADMIN_USER="${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
KC_ADMIN_PASSWORD="${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
KC_PUBLIC_PORT="${KEYCLOAK_PORT:-8081}"
KC_BASE_PATH_VALUE="${KC_BASE_PATH_VALUE:-http://localhost:${KC_PUBLIC_PORT}}"

# Direct script to get Keycloak credentials
echo "=== GETTING KEYCLOAK CREDENTIALS (DIRECT COMMAND) ==="

# Check if the container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${KEYCLOAK_CONTAINER_NAME}$"; then
  echo "Error: Container ${KEYCLOAK_CONTAINER_NAME} is not running"
    exit 1
fi

echo "Executing direct command in the container..."

# Execute direct command to retrieve only the secret
CLIENT_SECRET=$(docker exec "$KEYCLOAK_CONTAINER_NAME" bash -c "
  /opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user '$KC_ADMIN_USER' --password '$KC_ADMIN_PASSWORD' > /dev/null 2>&1
  CLIENT_ID=\$(/opt/keycloak/bin/kcadm.sh get clients -r cbweb3 --fields id,clientId | jq -r '.[] | select(.clientId==\"cbweb3-client-id\") | .id')
  /opt/keycloak/bin/kcadm.sh get clients/\$CLIENT_ID/client-secret -r cbweb3 | jq -r '.value'
")

if [[ -z "$CLIENT_SECRET" || "$CLIENT_SECRET" == "null" ]]; then
    echo "Error: unable to fetch client secret"
    exit 1
fi

echo "Recreating $ENV_FILE ..."
rm -f "$ENV_FILE"
cat > "$ENV_FILE" <<EOF
KC_BASE_PATH=$KC_BASE_PATH_VALUE
KC_CLIENT_ID=cbweb3-client-id
KC_CLIENT_SECRET=$CLIENT_SECRET
KC_REALM=cbweb3
KC_SKIP_TLS_VERIFY=true
EOF

echo "Done. .env.keycloak updated successfully."
