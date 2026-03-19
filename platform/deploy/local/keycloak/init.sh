#!/bin/bash
set -e

ENV_OUTPUT_DIR="${KEYCLOAK_ENV_OUTPUT_DIR:-${KEYCLOACK_ENV_OUTPUT_DIR:-/opt/keycloak/host/backend/config}}"
ENV_OUTPUT_FILE_SPOKE_A="${KEYCLOAK_ENV_OUTPUT_FILE_SPOKE_A:-$ENV_OUTPUT_DIR/.env.keycloak.spoke-a}"
ENV_OUTPUT_FILE_SPOKE_B="${KEYCLOAK_ENV_OUTPUT_FILE_SPOKE_B:-$ENV_OUTPUT_DIR/.env.keycloak.spoke-b}"
ENV_OUTPUT_FILE_SPOKE_A_ALIAS="${KEYCLOACK_ENV_OUTPUT_FILE_SPOKE_A:-$ENV_OUTPUT_DIR/.env.keycloack.spoke-a}"
ENV_OUTPUT_FILE_SPOKE_B_ALIAS="${KEYCLOACK_ENV_OUTPUT_FILE_SPOKE_B:-$ENV_OUTPUT_DIR/.env.keycloack.spoke-b}"
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
  local realm_name="$1"
  local client_id="$2"
  local env_output_file="$3"
  local env_output_file_alias="$4"

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

  echo "Recreating ${env_output_file} with latest credentials..."
  mkdir -p "$(dirname "$env_output_file")"
  rm -f "$env_output_file"
  cat > "$env_output_file" <<EOF
KC_BASE_PATH=$KC_BASE_PATH_VALUE
KC_CLIENT_ID=$client_id
KC_CLIENT_SECRET=$client_secret
KC_REALM=$realm_name
KC_SKIP_TLS_VERIFY=true
EOF

  if [[ -n "$env_output_file_alias" ]]; then
    echo "Recreating ${env_output_file_alias} (compat alias)..."
    mkdir -p "$(dirname "$env_output_file_alias")"
    rm -f "$env_output_file_alias"
    cp "$env_output_file" "$env_output_file_alias"
  fi
}

create_realm_and_client "cbweb3-spoke-a" "cbweb3-spoke-a-client" "$ENV_OUTPUT_FILE_SPOKE_A" "$ENV_OUTPUT_FILE_SPOKE_A_ALIAS"
create_realm_and_client "cbweb3-spoke-b" "cbweb3-spoke-b-client" "$ENV_OUTPUT_FILE_SPOKE_B" "$ENV_OUTPUT_FILE_SPOKE_B_ALIAS"

echo -e "\nConfiguração concluída. Keycloak está em execução."

wait $KEYCLOAK_PID
