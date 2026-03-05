#!/bin/bash
set -e

ENV_OUTPUT_FILE="${KEYCLOAK_ENV_OUTPUT_FILE:-/opt/keycloak/host/.env.keycloak}"
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

# Create the realm "cbweb3" if it does not exist
if ! /opt/keycloak/bin/kcadm.sh get realms/cbweb3 > /dev/null 2>&1; then
  /opt/keycloak/bin/kcadm.sh create realms -s realm=cbweb3 -s enabled=true
fi

# Create the client "cbweb3-client-id" if it does not exist
if ! /opt/keycloak/bin/kcadm.sh get clients -r cbweb3 --fields clientId | jq -e '.[] | select(.clientId=="cbweb3-client-id")' > /dev/null 2>&1; then
  /opt/keycloak/bin/kcadm.sh create clients -r cbweb3 \
    -s clientId=cbweb3-client-id \
    -s name='${client_cbweb3-client-id}' \
    -s enabled=true \
    -s alwaysDisplayInConsole=true \
    -s serviceAccountsEnabled=true \
    -s authorizationServicesEnabled=true \
    -s standardFlowEnabled=true \
    -s directAccessGrantsEnabled=true
fi

echo -e "\n\nAssigning role 'manage-users' to the service account..."

# Uses the jq to search IDs
echo -n "Rescuing client ID from cbweb3-client-id... "
CLIENT_ID=$(
  /opt/keycloak/bin/kcadm.sh get clients -r cbweb3 --fields id,clientId \
    | jq -r '.[] | select(.clientId=="cbweb3-client-id") | .id'
)

echo $CLIENT_ID

echo -n "Rescuing Service Account Username... "
SERVICE_USERNAME=$(
  /opt/keycloak/bin/kcadm.sh get clients/$CLIENT_ID/service-account-user -r cbweb3 \
    | jq -r '.username'
)

echo $SERVICE_USERNAME

if ! /opt/keycloak/bin/kcadm.sh get users -r cbweb3 -q username="$SERVICE_USERNAME" \
  | jq -e '.[0].id as $uid | $uid != null' > /dev/null 2>&1; then
  echo "Warning: service account user not found to grant role"
else
  /opt/keycloak/bin/kcadm.sh add-roles \
    --rolename manage-users \
    --uusername "$SERVICE_USERNAME" \
    --cclientid realm-management \
    -r cbweb3 || true
fi

echo -n "Fetching client secret... "
CLIENT_SECRET=$(
  /opt/keycloak/bin/kcadm.sh get clients/$CLIENT_ID/client-secret -r cbweb3 \
    | jq -r '.value'
)
echo "done"

echo "Recreating $ENV_OUTPUT_FILE with latest credentials..."
rm -f "$ENV_OUTPUT_FILE"
cat > "$ENV_OUTPUT_FILE" <<EOF
KC_BASE_PATH=$KC_BASE_PATH_VALUE
KC_CLIENT_ID=cbweb3-client-id
KC_CLIENT_SECRET=$CLIENT_SECRET
KC_REALM=cbweb3
KC_SKIP_TLS_VERIFY=true
EOF

echo -e "\nConfiguração concluída. Keycloak está em execução."

wait $KEYCLOAK_PID
