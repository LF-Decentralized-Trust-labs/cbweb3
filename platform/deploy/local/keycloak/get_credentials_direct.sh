#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
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
    DOMAIN_INFRA_ENV_FILE_EXAMPLE="$PROJECT_ROOT/backend/config/.env.infra.spoke-a.example"
    ;;
  b)
    DOMAIN_INFRA_ENV_FILE="$PROJECT_ROOT/backend/config/.env.infra.spoke-b"
    DOMAIN_INFRA_ENV_FILE_EXAMPLE="$PROJECT_ROOT/backend/config/.env.infra.spoke-b.example"
    ;;
  *)
    echo "Error: invalid spoke '$TARGET_SPOKE'. Use 'a' or 'b'."
    exit 1
    ;;
esac

source_if_exists() {
  local env_file="$1"
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
}

source_if_exists "$PROJECT_ROOT/backend/config/.env.infra.hub.example"
source_if_exists "$PROJECT_ROOT/backend/config/.env.infra.hub"
source_if_exists "$DOMAIN_INFRA_ENV_FILE_EXAMPLE"

if [[ -f "$DOMAIN_INFRA_ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$DOMAIN_INFRA_ENV_FILE"
  set +a
fi

source_if_exists "$BOOTSTRAP_ENV_FILE"
source_if_exists "$BOOTSTRAP_ENV_FILE_ALIAS"

escape_sed_replacement() {
  printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

copy_example_to_env() {
  local example_file="$1"
  local target_file="$2"
  mkdir -p "$(dirname "$target_file")"
  if [[ -f "$example_file" ]]; then
    cp "$example_file" "$target_file"
  else
    : > "$target_file"
  fi
}

set_env_var() {
  local target_file="$1"
  local key="$2"
  local value="$3"
  local escaped_value
  escaped_value="$(escape_sed_replacement "$value")"

  if grep -Eq "^#?${key}=" "$target_file"; then
    sed -i -E "s|^#?${key}=.*|${key}=${escaped_value}|" "$target_file"
  else
    printf '%s=%s\n' "$key" "$value" >> "$target_file"
  fi
}

KEYCLOAK_CONTAINER_NAME="${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"
KC_ADMIN_USER="${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
KC_ADMIN_PASSWORD="${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
KC_PUBLIC_PORT="${KEYCLOAK_PORT:-8081}"
KC_BASE_PATH_VALUE="${KC_BASE_PATH_VALUE:-http://localhost:${KC_PUBLIC_PORT}}"
HUB_INFRA_ENV_FILE="$PROJECT_ROOT/backend/config/.env.infra.hub"
HUB_INFRA_ENV_FILE_EXAMPLE="$PROJECT_ROOT/backend/config/.env.infra.hub.example"

case "$TARGET_SPOKE" in
  a)
    KC_REALM="${KC_REALM:-cbweb3-spoke-a}"
    KC_CLIENT_ID_VALUE="${KC_CLIENT_ID:-cbweb3-spoke-a-client}"
    ;;
  b)
    KC_REALM="${KC_REALM:-cbweb3-spoke-b}"
    KC_CLIENT_ID_VALUE="${KC_CLIENT_ID:-cbweb3-spoke-b-client}"
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

echo "Waiting for Keycloak admin API readiness..."
attempt=0
max_attempts=40
until docker exec "$KEYCLOAK_CONTAINER_NAME" bash -c "curl -sf http://localhost:8080/realms/master >/dev/null"; do
  attempt=$((attempt + 1))
  if [[ $attempt -ge $max_attempts ]]; then
    echo "Error: Keycloak admin API did not become ready in time"
    exit 1
  fi
  sleep 2
done

# Execute direct command to retrieve only the secret
CLIENT_SECRET=$(docker exec "$KEYCLOAK_CONTAINER_NAME" bash -c "
  /opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user '$KC_ADMIN_USER' --password '$KC_ADMIN_PASSWORD' > /dev/null 2>&1
  CLIENT_ID=\$(/opt/keycloak/bin/kcadm.sh get clients -r '$KC_REALM' --fields id,clientId | jq -r '.[] | select(.clientId==\"$KC_CLIENT_ID_VALUE\") | .id')
  if [ -z \"\$CLIENT_ID\" ] || [ \"\$CLIENT_ID\" = \"null\" ]; then
    exit 2
  fi
  /opt/keycloak/bin/kcadm.sh get clients/\$CLIENT_ID/client-secret -r '$KC_REALM' | jq -r '.value'
")

if [[ -z "$CLIENT_SECRET" || "$CLIENT_SECRET" == "null" ]]; then
    echo "Error: unable to fetch client secret"
    exit 1
fi

echo "Recreating $DOMAIN_INFRA_ENV_FILE from template and injecting Keycloak values..."
copy_example_to_env "$DOMAIN_INFRA_ENV_FILE_EXAMPLE" "$DOMAIN_INFRA_ENV_FILE"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KEYCLOAK_CONTAINER_NAME" "$KEYCLOAK_CONTAINER_NAME"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KEYCLOAK_PORT" "$KC_PUBLIC_PORT"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KEYCLOAK_ENV_OUTPUT_DIR" "${KEYCLOAK_ENV_OUTPUT_DIR:-$PROJECT_ROOT/backend/config}"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_BOOTSTRAP_ADMIN_USERNAME" "$KC_ADMIN_USER"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_BOOTSTRAP_ADMIN_PASSWORD" "$KC_ADMIN_PASSWORD"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_BASE_PATH" "$KC_BASE_PATH_VALUE"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_REALM" "$KC_REALM"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_CLIENT_ID" "$KC_CLIENT_ID_VALUE"
set_env_var "$DOMAIN_INFRA_ENV_FILE" "KC_CLIENT_SECRET" "$CLIENT_SECRET"

echo "Recreating $HUB_INFRA_ENV_FILE from template and injecting shared Keycloak values..."
copy_example_to_env "$HUB_INFRA_ENV_FILE_EXAMPLE" "$HUB_INFRA_ENV_FILE"
set_env_var "$HUB_INFRA_ENV_FILE" "KEYCLOAK_CONTAINER_NAME" "$KEYCLOAK_CONTAINER_NAME"
set_env_var "$HUB_INFRA_ENV_FILE" "KEYCLOAK_PORT" "$KC_PUBLIC_PORT"
set_env_var "$HUB_INFRA_ENV_FILE" "KEYCLOAK_ENV_OUTPUT_DIR" "${KEYCLOAK_ENV_OUTPUT_DIR:-$PROJECT_ROOT/backend/config}"
set_env_var "$HUB_INFRA_ENV_FILE" "KC_BOOTSTRAP_ADMIN_USERNAME" "$KC_ADMIN_USER"
set_env_var "$HUB_INFRA_ENV_FILE" "KC_BOOTSTRAP_ADMIN_PASSWORD" "$KC_ADMIN_PASSWORD"
set_env_var "$HUB_INFRA_ENV_FILE" "KC_BASE_PATH" "$KC_BASE_PATH_VALUE"

echo "Done. ${DOMAIN_INFRA_ENV_FILE##*/} updated successfully."
