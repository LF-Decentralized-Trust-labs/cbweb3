#!/bin/bash
set -euo pipefail

PROJECT_ROOT="${PROJECT_ROOT:-/opt/keycloak/host}"
CONFIG_DIR="$PROJECT_ROOT/backend/config"

INFRA_ENV_SPOKE_A="$CONFIG_DIR/.env.infra.spoke-a"
INFRA_ENV_SPOKE_A_EXAMPLE="$CONFIG_DIR/.env.infra.spoke-a.example"
INFRA_ENV_SPOKE_B="$CONFIG_DIR/.env.infra.spoke-b"
INFRA_ENV_SPOKE_B_EXAMPLE="$CONFIG_DIR/.env.infra.spoke-b.example"
INFRA_ENV_HUB="$CONFIG_DIR/.env.infra.hub"
INFRA_ENV_HUB_EXAMPLE="$CONFIG_DIR/.env.infra.hub.example"

source_if_exists() {
  local env_file="$1"
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$env_file"
    set +a
  fi
}

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

load_hub_defaults() {
  source_if_exists "$INFRA_ENV_HUB_EXAMPLE"
  source_if_exists "$INFRA_ENV_HUB"
}

load_hub_defaults
KC_PUBLIC_PORT="${KEYCLOAK_PORT:-8081}"
KC_BASE_PATH_VALUE="${KC_BASE_PATH_VALUE:-http://localhost:${KC_PUBLIC_PORT}}"

# Start Keycloak in background
/opt/keycloak/bin/kc.sh start-dev &

# Save the PID to wait later
KEYCLOAK_PID=$!

# Wait for Keycloak to be ready
echo "Waiting for Keycloak to start..."
until curl -s http://localhost:8080/realms/master; do
  sleep 3
done

# Authenticate via CLI
/opt/keycloak/bin/kcadm.sh config credentials --server http://localhost:8080 --realm master --user "$KC_BOOTSTRAP_ADMIN_USERNAME" --password "$KC_BOOTSTRAP_ADMIN_PASSWORD"

create_realm_and_client() {
  local default_realm_name="$1"
  local default_client_id="$2"
  local domain_env_file="$3"
  local domain_env_file_example="$4"

  # Reset domain-scoped values and load defaults from .example first.
  unset KC_REALM KC_CLIENT_ID KC_BASE_PATH
  source_if_exists "$domain_env_file_example"
  source_if_exists "$domain_env_file"

  local realm_name="${KC_REALM:-$default_realm_name}"
  local client_id="${KC_CLIENT_ID:-$default_client_id}"
  local base_path_value="${KC_BASE_PATH:-$KC_BASE_PATH_VALUE}"

  echo -e "\n=== Provisioning realm '${realm_name}' and client '${client_id}' ==="

  if ! /opt/keycloak/bin/kcadm.sh get "realms/${realm_name}" > /dev/null 2>&1; then
    /opt/keycloak/bin/kcadm.sh create realms -s "realm=${realm_name}" -s enabled=true
  fi

  if ! /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields clientId | jq -e ".[] | select(.clientId==\"${client_id}\")" > /dev/null 2>&1; then
    /opt/keycloak/bin/kcadm.sh create clients -r "$realm_name" \
      -s "clientId=${client_id}" \
      -s "name=${client_id}" \
      -s enabled=true \
      -s alwaysDisplayInConsole=true \
      -s serviceAccountsEnabled=true \
      -s authorizationServicesEnabled=true \
      -s standardFlowEnabled=true \
      -s directAccessGrantsEnabled=true
  fi

  echo "Assigning role 'manage-users' to service account for ${client_id}..."
  local client_uuid
  client_uuid=$(
    /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields id,clientId \
      | jq -r ".[] | select(.clientId==\"${client_id}\") | .id"
  )

  if [[ -z "$client_uuid" ]]; then
    echo "Error: unable to resolve internal client ID for ${client_id} in realm ${realm_name}"
    exit 1
  fi

  local service_username
  service_username=$(
    /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/service-account-user" -r "$realm_name" \
      | jq -r '.username'
  )

  if [[ -z "$service_username" || "$service_username" == "null" ]]; then
    echo "Warning: service account user not found for client ${client_id}"
  elif ! /opt/keycloak/bin/kcadm.sh get users -r "$realm_name" -q "username=${service_username}" \
    | jq -e '.[0].id as $uid | $uid != null' > /dev/null 2>&1; then
    echo "Warning: service account user ${service_username} not found to grant role"
  else
    /opt/keycloak/bin/kcadm.sh add-roles \
      --rolename manage-users \
      --uusername "$service_username" \
      --cclientid realm-management \
      -r "$realm_name" || true
  fi

  local client_secret
  client_secret=$(
    /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/client-secret" -r "$realm_name" \
      | jq -r '.value'
  )

  if [[ -z "$client_secret" || "$client_secret" == "null" ]]; then
    echo "Error: unable to fetch client secret for ${client_id} in realm ${realm_name}"
    exit 1
  fi

  echo "Recreating ${domain_env_file} from template and injecting Keycloak values..."
  copy_example_to_env "$domain_env_file_example" "$domain_env_file"
  set_env_var "$domain_env_file" "KEYCLOAK_CONTAINER_NAME" "${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"
  set_env_var "$domain_env_file" "KEYCLOAK_PORT" "${KEYCLOAK_PORT:-8081}"
  set_env_var "$domain_env_file" "KEYCLOAK_ENV_OUTPUT_DIR" "${KEYCLOAK_ENV_OUTPUT_DIR:-$CONFIG_DIR}"
  set_env_var "$domain_env_file" "KC_BOOTSTRAP_ADMIN_USERNAME" "${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
  set_env_var "$domain_env_file" "KC_BOOTSTRAP_ADMIN_PASSWORD" "${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
  set_env_var "$domain_env_file" "KC_BASE_PATH" "$base_path_value"
  set_env_var "$domain_env_file" "KC_REALM" "$realm_name"
  set_env_var "$domain_env_file" "KC_CLIENT_ID" "$client_id"
  set_env_var "$domain_env_file" "KC_CLIENT_SECRET" "$client_secret"
}

refresh_hub_infra() {
  echo "Recreating ${INFRA_ENV_HUB} from template and injecting Keycloak values..."
  load_hub_defaults
  copy_example_to_env "$INFRA_ENV_HUB_EXAMPLE" "$INFRA_ENV_HUB"
  set_env_var "$INFRA_ENV_HUB" "KEYCLOAK_CONTAINER_NAME" "${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"
  set_env_var "$INFRA_ENV_HUB" "KEYCLOAK_PORT" "${KEYCLOAK_PORT:-8081}"
  set_env_var "$INFRA_ENV_HUB" "KEYCLOAK_ENV_OUTPUT_DIR" "${KEYCLOAK_ENV_OUTPUT_DIR:-$CONFIG_DIR}"
  set_env_var "$INFRA_ENV_HUB" "KC_BOOTSTRAP_ADMIN_USERNAME" "${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
  set_env_var "$INFRA_ENV_HUB" "KC_BOOTSTRAP_ADMIN_PASSWORD" "${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
  set_env_var "$INFRA_ENV_HUB" "KC_BASE_PATH" "${KC_BASE_PATH:-$KC_BASE_PATH_VALUE}"
}

create_realm_and_client \
  "cbweb3-spoke-a" \
  "cbweb3-spoke-a-client" \
  "$INFRA_ENV_SPOKE_A" \
  "$INFRA_ENV_SPOKE_A_EXAMPLE"

create_realm_and_client \
  "cbweb3-spoke-b" \
  "cbweb3-spoke-b-client" \
  "$INFRA_ENV_SPOKE_B" \
  "$INFRA_ENV_SPOKE_B_EXAMPLE"

refresh_hub_infra

echo -e "\nConfiguração concluída. Keycloak está em execução."

wait $KEYCLOAK_PID
