#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
ENV_OUTPUT_DIR="${KEYCLOAK_ENV_OUTPUT_DIR:-${KEYCLOACK_ENV_OUTPUT_DIR:-$PROJECT_ROOT/backend/config}}"
BOOTSTRAP_ENV_FILE="$PROJECT_ROOT/backend/config/.env.keycloak.bootstrap"
BOOTSTRAP_ENV_FILE_ALIAS="$PROJECT_ROOT/backend/config/.env.keycloack.bootstrap"
TARGET_SPOKE="a"

usage() {
  echo "Usage: $0 [--spoke a|b]"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --spoke)
      shift
      [[ $# -gt 0 ]] || usage
      TARGET_SPOKE="$1"
      ;;
    *)
      usage
      ;;
  esac
  shift
done

case "$TARGET_SPOKE" in
  a)
    DOMAIN_INFRA_ENV_FILE="$PROJECT_ROOT/backend/config/.env.infra.spoke-a"
    ;;
  b)
    DOMAIN_INFRA_ENV_FILE="$PROJECT_ROOT/backend/config/.env.infra.spoke-b"
    ;;
  *)
    echo "Error: invalid spoke '$TARGET_SPOKE'. Use 'a' or 'b'."
    exit 1
    ;;
esac

if [[ -f "$DOMAIN_INFRA_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$DOMAIN_INFRA_ENV_FILE"
  set +a
fi

if [[ -f "$BOOTSTRAP_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$BOOTSTRAP_ENV_FILE"
  set +a
fi

if [[ -f "$BOOTSTRAP_ENV_FILE_ALIAS" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$BOOTSTRAP_ENV_FILE_ALIAS"
  set +a
fi

KEYCLOAK_CONTAINER_NAME="${KEYCLOAK_CONTAINER_NAME:-keycloak}"
KC_ADMIN_USER="${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
KC_ADMIN_PASSWORD="${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
KC_PUBLIC_PORT="${KEYCLOAK_PORT:-8081}"
KC_BASE_PATH_VALUE="${KC_BASE_PATH_VALUE:-http://localhost:${KC_PUBLIC_PORT}}"

case "$TARGET_SPOKE" in
  a)
    KC_REALM="cbweb3-spoke-a"
    KC_CLIENT_ID_VALUE="cbweb3-spoke-a-client"
    ENV_FILE="$ENV_OUTPUT_DIR/.env.keycloak.spoke-a"
    ENV_FILE_ALIAS="$ENV_OUTPUT_DIR/.env.keycloack.spoke-a"
    ;;
  b)
    KC_REALM="cbweb3-spoke-b"
    KC_CLIENT_ID_VALUE="cbweb3-spoke-b-client"
    ENV_FILE="$ENV_OUTPUT_DIR/.env.keycloak.spoke-b"
    ENV_FILE_ALIAS="$ENV_OUTPUT_DIR/.env.keycloack.spoke-b"
    ;;
  *)
    echo "Error: invalid spoke '$TARGET_SPOKE'. Use 'a' or 'b'."
    exit 1
    ;;
esac

# Direct script to get Keycloak credentials
echo "=== GETTING KEYCLOAK CREDENTIALS (SPOKE-${TARGET_SPOKE^^}) ==="

# Check if the container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${KEYCLOAK_CONTAINER_NAME}$"; then
  echo "Error: Container ${KEYCLOAK_CONTAINER_NAME} is not running"
    exit 1
fi

echo "Executing direct command in the container..."

# Execute direct command to retrieve only the secret
CLIENT_SECRET=$(docker exec "$KEYCLOAK_CONTAINER_NAME" bash -c "
  /opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user '$KC_ADMIN_USER' --password '$KC_ADMIN_PASSWORD' > /dev/null 2>&1
  CLIENT_ID=\$(/opt/keycloak/bin/kcadm.sh get clients -r '$KC_REALM' --fields id,clientId | jq -r '.[] | select(.clientId==\"$KC_CLIENT_ID_VALUE\") | .id')
  /opt/keycloak/bin/kcadm.sh get clients/\$CLIENT_ID/client-secret -r '$KC_REALM' | jq -r '.value'
")

if [[ -z "$CLIENT_SECRET" || "$CLIENT_SECRET" == "null" ]]; then
    echo "Error: unable to fetch client secret"
    exit 1
fi

echo "Recreating $ENV_FILE ..."
mkdir -p "$(dirname "$ENV_FILE")"
rm -f "$ENV_FILE"
cat > "$ENV_FILE" <<EOF
KC_BASE_PATH=$KC_BASE_PATH_VALUE
KC_CLIENT_ID=$KC_CLIENT_ID_VALUE
KC_CLIENT_SECRET=$CLIENT_SECRET
KC_REALM=$KC_REALM
KC_SKIP_TLS_VERIFY=true
EOF

if [[ -n "$ENV_FILE_ALIAS" ]]; then
  echo "Recreating $ENV_FILE_ALIAS (compat alias)..."
  rm -f "$ENV_FILE_ALIAS"
  cp "$ENV_FILE" "$ENV_FILE_ALIAS"
fi

echo "Done. ${ENV_FILE##*/} updated successfully."
