#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

PROJECT_ROOT="${PROJECT_ROOT:-/opt/keycloak/host}"
CONFIG_DIR="$PROJECT_ROOT/backend/config"

INFRA_ENV_BANK_A="$CONFIG_DIR/.env.infra.bank-a"
INFRA_ENV_BANK_A_EXAMPLE="$CONFIG_DIR/.env.infra.bank-a.example"
INFRA_ENV_BANK_B="$CONFIG_DIR/.env.infra.bank-b"
INFRA_ENV_BANK_B_EXAMPLE="$CONFIG_DIR/.env.infra.bank-b.example"
INFRA_ENV_CENTRAL_BANK_A="$CONFIG_DIR/.env.infra.central-bank-a"
INFRA_ENV_CENTRAL_BANK_A_EXAMPLE="$CONFIG_DIR/.env.infra.central-bank-a.example"
INFRA_ENV_CENTRAL_BANK_B="$CONFIG_DIR/.env.infra.central-bank-b"
INFRA_ENV_CENTRAL_BANK_B_EXAMPLE="$CONFIG_DIR/.env.infra.central-bank-b.example"
INFRA_ENV_MLP="$CONFIG_DIR/.env.infra.mlp"
INFRA_ENV_MLP_EXAMPLE="$CONFIG_DIR/.env.infra.mlp.example"

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

load_bank_a_defaults() {
  source_if_exists "$INFRA_ENV_BANK_A_EXAMPLE"
  source_if_exists "$INFRA_ENV_BANK_A"
}

load_bank_a_defaults
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

  # Access-token lifespan. 300s is Keycloak's own default, and the realms the
  # toolkit provisions already run at it; the portals renew silently (proactive
  # refresh + a 401-retry interceptor), so a short-lived token costs no session.
  # This was 21600 — six hours — on the `make *.up` path, which is the finding
  # R1-10.7 / R2-4.11 calls out: a leaked or logged bearer stayed usable for a
  # working day. Overridable, but never back to a value measured in hours.
  local token_lifespan="${KC_ACCESS_TOKEN_LIFESPAN:-300}"
  /opt/keycloak/bin/kcadm.sh update "realms/${realm_name}" \
    -s "accessTokenLifespan=${token_lifespan}"

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

  add_attribute_mappers "$realm_name" "$client_uuid"

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

  # If the .example (or .env) file defines KC_CLIENT_SECRET, push it into Keycloak
  # so the secret stays stable across re-deployments. Otherwise read the generated one.
  local desired_secret="${KC_CLIENT_SECRET:-}"
  if [[ -n "$desired_secret" ]]; then
    /opt/keycloak/bin/kcadm.sh update "clients/${client_uuid}" -r "$realm_name" \
      -s "secret=${desired_secret}"
    echo "  Using fixed client secret for ${client_id}."
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

  if [[ ! -f "$domain_env_file" ]]; then
    echo "Creating ${domain_env_file} from template and injecting Keycloak values..."
    copy_example_to_env "$domain_env_file_example" "$domain_env_file"
  else
    echo "Updating Keycloak values in existing ${domain_env_file} (preserving non-Keycloak settings)..."
  fi
  set_env_var "$domain_env_file" "KEYCLOAK_CONTAINER_NAME" "${KEYCLOAK_CONTAINER_NAME:-cbweb3-keycloak}"
  set_env_var "$domain_env_file" "KEYCLOAK_PORT" "${KEYCLOAK_PORT:-8081}"
  set_env_var "$domain_env_file" "KEYCLOAK_ENV_OUTPUT_DIR" "${KEYCLOAK_ENV_OUTPUT_DIR:-$CONFIG_DIR}"
  set_env_var "$domain_env_file" "KC_BOOTSTRAP_ADMIN_USERNAME" "${KC_BOOTSTRAP_ADMIN_USERNAME:-admin}"
  set_env_var "$domain_env_file" "KC_BOOTSTRAP_ADMIN_PASSWORD" "${KC_BOOTSTRAP_ADMIN_PASSWORD:-admin}"
  set_env_var "$domain_env_file" "KC_BASE_PATH" "$base_path_value"
  set_env_var "$domain_env_file" "KC_REALM" "$realm_name"
  set_env_var "$domain_env_file" "KC_CLIENT_ID" "$client_id"
  set_env_var "$domain_env_file" "KC_CLIENT_SECRET" "$client_secret"

  # GOVERNANCE_USER_ID is only needed by central banks (compliance governance bootstrap).
  # Commercial bank .env templates do not define it; only write when present.
  # We store the Keycloak service-account UUID (not the username) because ValidateToken
  # enriches BankID by looking up participants using the JWT "sub" claim (UUID).
  if grep -Eq "^#?GOVERNANCE_USER_ID=" "$domain_env_file"; then
    local sa_uuid
    sa_uuid=$(
      /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/service-account-user" -r "$realm_name" \
        | jq -r '.id'
    )
    set_env_var "$domain_env_file" "GOVERNANCE_USER_ID" "${sa_uuid:-service-account-${client_id}}"
  fi

  # GOVERNANCE_BANK_CODE is the bank_code stored in the compliance participant record
  # for the CB service account. Derived from the realm name (e.g. central-bank-a).
  if grep -Eq "^#?GOVERNANCE_BANK_CODE=" "$domain_env_file"; then
    set_env_var "$domain_env_file" "GOVERNANCE_BANK_CODE" "${realm_name}"
  fi
}

create_platform_roles() {
  local realm_name="$1"
  shift
  local roles=("$@")
  echo -e "\n=== Provisioning roles in realm '${realm_name}' ==="
  for role in "${roles[@]}"; do
    if ! /opt/keycloak/bin/kcadm.sh get roles -r "$realm_name" --fields name \
        | jq -e ".[] | select(.name==\"${role}\")" > /dev/null 2>&1; then
      /opt/keycloak/bin/kcadm.sh create roles -r "$realm_name" -s "name=${role}"
      echo "  Created role: ${role}"
    else
      echo "  Role already exists: ${role}"
    fi
  done
}

add_attribute_mappers() {
  local realm_name="$1"
  local client_uuid="$2"
  local attrs=("wallet" "country" "bank_id" "privacy_group")
  echo "  Adding attribute mappers for client in realm '${realm_name}'..."
  for attr in "${attrs[@]}"; do
    /opt/keycloak/bin/kcadm.sh create \
      "clients/${client_uuid}/protocol-mappers/models" \
      -r "$realm_name" \
      -s "name=${attr}" \
      -s protocol=openid-connect \
      -s protocolMapper=oidc-usermodel-attribute-mapper \
      -s consentRequired=false \
      -s "config.user.attribute=${attr}" \
      -s "config.claim.name=${attr}" \
      -s "config.jsonType.label=String" \
      -s "config.access.token.claim=true" \
      -s "config.id.token.claim=true" \
      -s "config.userinfo.token.claim=true" \
      -s "config.multivalued=false" \
      2>/dev/null || echo "    Mapper '${attr}' may already exist — skipping"
  done
}

assign_governance_role_to_service_account() {
  local realm_name="$1"
  local client_id="$2"
  echo "  Assigning ROLE_GOVERNANCE to service account of '${client_id}' in realm '${realm_name}'..."
  local client_uuid
  client_uuid=$(
    /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields id,clientId \
      | jq -r ".[] | select(.clientId==\"${client_id}\") | .id"
  )
  if [[ -z "$client_uuid" ]]; then
    echo "  Warning: client '${client_id}' not found in realm '${realm_name}', skipping"
    return 0
  fi
  local service_username
  service_username=$(
    /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/service-account-user" -r "$realm_name" \
      | jq -r '.username'
  )
  if [[ -n "$service_username" && "$service_username" != "null" ]]; then
    /opt/keycloak/bin/kcadm.sh add-roles \
      --rolename ROLE_GOVERNANCE \
      --uusername "$service_username" \
      -r "$realm_name" || true
    echo "  Assigned ROLE_GOVERNANCE to ${service_username}"
  else
    echo "  Warning: service account for '${client_id}' not found, skipping"
  fi
}

# create_extra_client_in_realm provisions an additional OAuth2 client in an existing realm.
# Used to give each portal (governance, treasury, supervisor) its own credential pair.
# The secret is logged so operators can copy it (or pass a fixed one via the 3rd argument).
create_extra_client_in_realm() {
  local realm_name="$1"
  local client_id="$2"
  local desired_secret="${3:-}"

  echo -e "\n=== Provisioning extra client '${client_id}' in realm '${realm_name}' ==="

  if ! /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields clientId \
      | jq -e ".[] | select(.clientId==\"${client_id}\")" > /dev/null 2>&1; then
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

  local client_uuid
  client_uuid=$(
    /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields id,clientId \
      | jq -r ".[] | select(.clientId==\"${client_id}\") | .id"
  )
  if [[ -z "$client_uuid" ]]; then
    echo "Error: unable to resolve internal client ID for ${client_id} in realm ${realm_name}"
    return 1
  fi

  local secret
  if [[ -n "$desired_secret" ]]; then
    /opt/keycloak/bin/kcadm.sh update "clients/${client_uuid}" -r "$realm_name" \
      -s "secret=${desired_secret}"
    secret="$desired_secret"
  else
    secret=$(
      /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/client-secret" -r "$realm_name" \
        | jq -r '.value'
    )
  fi

  echo "  Client '${client_id}' ready. Secret: ${secret}"
}

# assign_scenariob_roles_to_service_account assigns one or more realm roles
# to the service account of a given client. Used to grant commercial_bank
# or central_bank roles so that client_credentials tokens carry them.
assign_scenariob_roles_to_service_account() {
  local realm_name="$1"
  local client_id="$2"
  shift 2
  local roles=("$@")
  local client_uuid
  client_uuid=$(
    /opt/keycloak/bin/kcadm.sh get clients -r "$realm_name" --fields id,clientId \
      | jq -r ".[] | select(.clientId==\"${client_id}\") | .id"
  )
  if [[ -z "$client_uuid" ]]; then
    echo "  Warning: client '${client_id}' not found in realm '${realm_name}', skipping role assign"
    return 0
  fi
  local service_username
  service_username=$(
    /opt/keycloak/bin/kcadm.sh get "clients/${client_uuid}/service-account-user" -r "$realm_name" \
      | jq -r '.username'
  )
  if [[ -n "$service_username" && "$service_username" != "null" ]]; then
    for role in "${roles[@]}"; do
      /opt/keycloak/bin/kcadm.sh add-roles \
        --rolename "$role" \
        --uusername "$service_username" \
        -r "$realm_name" || true
      echo "  Assigned ${role} to ${service_username}"
    done
  else
    echo "  Warning: service account for '${client_id}' not found, skipping Scenario B roles"
  fi
}

# Roles for commercial banks (bank-a, bank-b).
# Scenario B adds the canonical `commercial_bank` realm role (FR-030 / FR-056) that the
# api-gateway RBAC middleware inspects in realm_access.roles for Liquidity Taker flows.
BANK_ROLES=(
  ROLE_COMMERCIAL_BANK ROLE_TREASURY ROLE_SUPERVISOR ROLE_NOC ROLE_GOVERNANCE_OFFICER ROLE_GOVERNANCE
  commercial_bank
)

# Roles for the central bank (superset: includes governance authority).
# Scenario B adds the canonical `central_bank` realm role (FR-030 / FR-056) required by
# Circuit Breaker pause/resume and Master Viewing Key disclosure endpoints.
CENTRAL_BANK_ROLES=(
  ROLE_COMMERCIAL_BANK ROLE_TREASURY ROLE_SUPERVISOR ROLE_NOC ROLE_GOVERNANCE_OFFICER ROLE_GOVERNANCE
  central_bank hub_operator
)

create_realm_and_client \
  "bank-a" \
  "bank-a-client" \
  "$INFRA_ENV_BANK_A" \
  "$INFRA_ENV_BANK_A_EXAMPLE"
create_platform_roles "bank-a" "${BANK_ROLES[@]}"
assign_governance_role_to_service_account "bank-a" "bank-a-client"
assign_scenariob_roles_to_service_account "bank-a" "bank-a-client" "commercial_bank"
set_env_var "$INFRA_ENV_BANK_A" "SPOKE_NETWORK"       "spoke-a"
set_env_var "$INFRA_ENV_BANK_A" "NATIVE_ASSET_SYMBOL" "tCeBM_BRL"
set_env_var "$INFRA_ENV_BANK_A" "W_TOKEN_ADDRESS"     "0xf204a4ef082f5c04bb89f7d5e6568b796096735a"

create_realm_and_client \
  "bank-b" \
  "bank-b-client" \
  "$INFRA_ENV_BANK_B" \
  "$INFRA_ENV_BANK_B_EXAMPLE"
create_platform_roles "bank-b" "${BANK_ROLES[@]}"
assign_governance_role_to_service_account "bank-b" "bank-b-client"
assign_scenariob_roles_to_service_account "bank-b" "bank-b-client" "commercial_bank"
set_env_var "$INFRA_ENV_BANK_B" "SPOKE_NETWORK"       "spoke-b"
set_env_var "$INFRA_ENV_BANK_B" "NATIVE_ASSET_SYMBOL" "tCeBM_ARS"
set_env_var "$INFRA_ENV_BANK_B" "W_TOKEN_ADDRESS"     "0x82d50ad3c1091866e258fd0f1a7cc9674609d254"

create_realm_and_client \
  "central-bank-a" \
  "central-bank-a-client" \
  "$INFRA_ENV_CENTRAL_BANK_A" \
  "$INFRA_ENV_CENTRAL_BANK_A_EXAMPLE"
create_platform_roles "central-bank-a" "${CENTRAL_BANK_ROLES[@]}"
assign_governance_role_to_service_account "central-bank-a" "central-bank-a-client"
assign_scenariob_roles_to_service_account "central-bank-a" "central-bank-a-client" "central_bank"
# Supervisor portal: dedicated client with its own credentials → ROLE_SUPERVISOR only.
create_extra_client_in_realm "central-bank-a" "central-bank-a-supervisor-client" "central-bank-a-supervisor-local-secret"
assign_scenariob_roles_to_service_account "central-bank-a" "central-bank-a-supervisor-client" ROLE_SUPERVISOR

create_realm_and_client \
  "central-bank-b" \
  "central-bank-b-client" \
  "$INFRA_ENV_CENTRAL_BANK_B" \
  "$INFRA_ENV_CENTRAL_BANK_B_EXAMPLE"
create_platform_roles "central-bank-b" "${CENTRAL_BANK_ROLES[@]}"
assign_governance_role_to_service_account "central-bank-b" "central-bank-b-client"
assign_scenariob_roles_to_service_account "central-bank-b" "central-bank-b-client" "central_bank"
# Supervisor portal: dedicated client with its own credentials → ROLE_SUPERVISOR only.
create_extra_client_in_realm "central-bank-b" "central-bank-b-supervisor-client" "central-bank-b-supervisor-local-secret"
assign_scenariob_roles_to_service_account "central-bank-b" "central-bank-b-supervisor-client" ROLE_SUPERVISOR

# ── MLP (Multilateral Liquidity Provider — Path B: realm próprio, gateway próprio) ─────────
# Ativado por ENABLE_MLP=true. Cria realm 'mlp', client 'mlp-client' e role 'mlp'.
# O auth-mlp valida tokens deste realm via JWKS próprio.
if [[ "${ENABLE_MLP:-false}" == "true" ]]; then
  echo -e "\n=== [MLP] Provisioning MLP realm, client and role ==="
  MLP_ROLES=(mlp)
  create_realm_and_client \
    "mlp" \
    "mlp-client" \
    "$INFRA_ENV_MLP" \
    "$INFRA_ENV_MLP_EXAMPLE"
  create_platform_roles "mlp" "${MLP_ROLES[@]}"
  assign_scenariob_roles_to_service_account "mlp" "mlp-client" "mlp"
  echo "[MLP] Provisioning complete. .env.infra.mlp written."
else
  echo -e "\n=== [MLP] Skipped (ENABLE_MLP != true) ==="
fi

echo "KEYCLOAK_INIT_DONE"
echo -e "\nConfiguração concluída. Keycloak está em execução."

wait $KEYCLOAK_PID
