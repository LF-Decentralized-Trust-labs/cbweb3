#!/usr/bin/env bash
# tryout-spoke-b-bank-d.sh — Onboard a commercial bank (bank-d) to the Central Bank
#
# Dual-entity 3-phase PKI + Blockchain onboarding flow.
# The script simulates both sides (Bank-D operator and CB governance) and
# exercises the full round-trip across two API Gateways.
#
# Flow (7 steps):
#
#   Step 1  Bank-D operator login (Bank-D Keycloak)
#   Step 2  Credential Request via Bank-D proxy -> CB public endpoint
#   Step 3  Central Bank governance login (CB Keycloak)
#   Step 4  Approve KYC (CB governance)
#   Step 5  Polling via Bank-D proxy (discovers pop_nonce)
#   Step 6  Complete Onboarding via Bank-D proxy (PoP + wallet bind)
#   Step 7  PKI Login directly on CB (validation)
#
# Prerequisites:
#   - Both stacks running: make dev.up (or dev.up-bank-d + dev.up-central-bank-b)
#   - curl, jq
#   - backend/config/.env.infra.bank-d    with KC_CLIENT_SECRET
#   - backend/config/.env.infra.central-bank-b with KC_CLIENT_SECRET
#
# Usage:
#   ./tryout-spoke-b-bank-d.sh
#
# Environment variables (optional):
#   BANK_URL   Bank-D API base         (default: http://localhost:58080/api/v1)
#   CB_URL     Central Bank API base   (default: http://localhost:60080/api/v1)
#   BANK_ENV   Bank-D .env file        (default: backend/config/.env.infra.bank-d)
#   CB_ENV     Central Bank .env file  (default: backend/config/.env.infra.central-bank-b)
#   PKI_DIR    Path to PKI files       (default: backend/config/pki)

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BANK_URL="${BANK_URL:-http://localhost:58080/api/v1}"
CB_URL="${CB_URL:-http://localhost:60080/api/v1}"
BANK_ENV="${BANK_ENV:-backend/config/.env.infra.bank-d}"
CB_ENV="${CB_ENV:-backend/config/.env.infra.central-bank-b}"
PKI_DIR="${PKI_DIR:-backend/config/pki}"

# Globals populated during execution
BANK_OPERATOR_TOKEN=""
CB_TOKEN=""
REQUEST_ID=""
BANK_USER_ID=""
WALLET_ADDRESS=""
POP_NONCE=""
CLIENT_SECRET=""
TX_HASH=""
BANK_TOKEN=""
PKI_LOGIN_ERROR=""

# ---------------------------------------------------------------------------
# Dependency checks
# ---------------------------------------------------------------------------

require_cmds() {
  for cmd in curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "ERROR: '$cmd' not found. Install it before continuing." >&2
      exit 1
    fi
  done
}

require_env_files() {
  if [ ! -f "$BANK_ENV" ]; then
    echo "ERROR: Bank-D env file '$BANK_ENV' not found." >&2
    exit 1
  fi
  if [ ! -f "$CB_ENV" ]; then
    echo "ERROR: Central Bank env file '$CB_ENV' not found." >&2
    exit 1
  fi
}


read_kc_secret() {
  local env_file=$1
  local secret
  secret=$(grep KC_CLIENT_SECRET "$env_file" | cut -d= -f2-)
  if [ -z "$secret" ]; then
    echo "ERROR: KC_CLIENT_SECRET not found in $env_file." >&2
    exit 1
  fi
  echo "$secret"
}

# ---------------------------------------------------------------------------
# Step 1 — Bank-D operator login (actor: Bank-D operator)
# Authenticates against Bank-D's Keycloak to get an operator token used
# for the protected proxy routes on Bank-D's gateway.
# ---------------------------------------------------------------------------

bank_operator_login() {
  local kc_secret login_resp
  kc_secret=$(read_kc_secret "$BANK_ENV")
  login_resp=$(curl -s -X POST "$BANK_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"bank-d-client\", \"clientSecret\": \"$kc_secret\"}")
  BANK_OPERATOR_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$BANK_OPERATOR_TOKEN" ]; then
    echo "ERROR: Bank-D operator login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Step 2 — Phase 1: Credential Request (actor: Bank-D operator)
# Calls Bank-D's protected proxy route which forwards to the Central Bank's
# public /onboarding/credential-request endpoint.
# ---------------------------------------------------------------------------

submit_credential_request() {
  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$BANK_URL/onboarding/initiate" \
    --cookie "access_token=$BANK_OPERATOR_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg inst "Bank D S.A." \
      --arg bank "d" \
      --arg country "BR" \
      --arg role "ROLE_COMMERCIAL_BANK" \
      --arg email "ops@bank-d.com.br" \
      --arg user "bank-d" \
      '{
        institution_name: $inst,
        bank_code: $bank,
        country: $country,
        role: $role,
        email: $email,
        username: $user
      }')" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "201" ]; then
    echo "ERROR: credential request failed (HTTP $code)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  REQUEST_ID=$(echo "$body" | jq -r '.request_id')
  BANK_USER_ID=$(echo "$body" | jq -r '.user_id')
  WALLET_ADDRESS=$(echo "$body" | jq -r '.wallet_address')
  echo "$body" | jq .
}

# ---------------------------------------------------------------------------
# Step 3 — Central Bank governance login (actor: CB governance)
# Authenticates against the Central Bank's Keycloak. Only needed for step 4
# (KYC approval). The commercial bank never holds this token.
# ---------------------------------------------------------------------------

cb_governance_login() {
  local kc_secret login_resp
  kc_secret=$(read_kc_secret "$CB_ENV")
  login_resp=$(curl -s -X POST "$CB_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"central-bank-b-client\", \"clientSecret\": \"$kc_secret\"}")
  CB_TOKEN=$(echo "$login_resp" | jq -r '.accessToken // empty')
  if [ -z "$CB_TOKEN" ]; then
    echo "ERROR: Central Bank governance login failed." >&2
    echo "Response: $login_resp" >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Step 4 — Phase 2: KYC Approval (actor: CB governance)
# Central Bank governance approves the commercial bank's documentation.
# ---------------------------------------------------------------------------

approve_kyc() {
  local resp status_val
  resp=$(curl -s -X POST "$CB_URL/compliance/approve-kyc" \
    --cookie "access_token=$CB_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg s "$BANK_USER_ID" '{subject: $s, reason: "KYC approved - tryout-spoke-b-bank-d"}')")
  status_val=$(echo "$resp" | jq -r '.status // empty')
  POP_NONCE=$(echo "$resp" | jq -r '.pop_nonce // empty')
  if [ "$status_val" != "KYC_APPROVED" ]; then
    echo "ERROR: KYC approval failed. Expected status=KYC_APPROVED, got '$status_val'." >&2
    echo "Response: $resp" >&2
    exit 1
  fi
  echo "$resp" | jq .
}

# ---------------------------------------------------------------------------
# Step 5 — Phase 2.5: Polling (actor: Bank-D operator)
# Calls Bank-D's protected proxy route to discover the KYC approval and
# retrieve the PoP nonce.
# ---------------------------------------------------------------------------

poll_onboarding_status() {
  local attempt max_attempts resp status_val nonce
  max_attempts=5
  for attempt in $(seq 1 "$max_attempts"); do
    resp=$(curl -s -X GET "$BANK_URL/onboarding/status/$REQUEST_ID" \
      --cookie "access_token=$BANK_OPERATOR_TOKEN" \
      -H "Content-Type: application/json")
    status_val=$(echo "$resp" | jq -r '.status // empty')
    nonce=$(echo "$resp" | jq -r '.pop_nonce // empty')
    if [ "$status_val" = "KYC_APPROVED" ] && [ -n "$nonce" ]; then
      POP_NONCE="$nonce"
      echo "$resp" | jq .
      return 0
    fi
    echo "  Attempt $attempt/$max_attempts: status=$status_val (waiting...)"
    sleep 2
  done
  echo "ERROR: polling timed out. Last status: $status_val" >&2
  exit 1
}

# ---------------------------------------------------------------------------
# Step 6 — Phase 3: Complete Onboarding (actor: Bank-D operator)
# Calls Bank-D's protected proxy route. The backend signs the PoP nonce
# with secp256k1 to prove wallet ownership.
# ---------------------------------------------------------------------------

complete_onboarding() {
  local tmp code body
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$BANK_URL/onboarding/complete" \
    --cookie "access_token=$BANK_OPERATOR_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg rid "$REQUEST_ID" \
      --arg uid "$BANK_USER_ID" \
      '{
        request_id: $rid,
        user_id: $uid
      }')" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp"); rm -f "$tmp"
  if [ "$code" != "200" ]; then
    echo "ERROR: complete onboarding failed (HTTP $code)." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  CLIENT_SECRET=$(echo "$body" | jq -r '.client_secret // empty')
  TX_HASH=$(echo "$body" | jq -r '.tx_hash // empty')
  WALLET_ADDRESS=$(echo "$body" | jq -r '.wallet_address // empty')
  BANK_TOKEN=$(echo "$body" | jq -r '.access_token // empty')
  PKI_LOGIN_ERROR=$(echo "$body" | jq -r '.pki_login_error // empty')
  if [ -z "$CLIENT_SECRET" ]; then
    echo "ERROR: incomplete response from complete onboarding." >&2
    echo "Response: $body" >&2
    exit 1
  fi
  echo "$body" | jq '{user_id, wallet_address, tx_hash, status, access_token: (.access_token // null | if . then "(present)" else null end), pki_login_error}'
}


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  require_cmds
  require_env_files

  echo ""
  echo "======================================================"
  echo "  Onboarding: bank-d -> Central-Bank-B (3-phase PKI)"
  echo "  Bank-D Gateway : $BANK_URL"
  echo "  Central Bank GW: $CB_URL"
  echo "======================================================"

  echo ""
  echo "--- Actor: BANK-D OPERATOR ---"
  echo "=== [1/7] Bank-D operator login ==="
  bank_operator_login
  echo "  BANK_OPERATOR_TOKEN: ${BANK_OPERATOR_TOKEN:0:60}..."

  echo ""
  echo "=== [2/7] Phase 1 — Credential Request (via Bank-D proxy) ==="
  echo "  Submitting credential request (CSR + blockchain key handled by backend)..."
  submit_credential_request
  echo "  request_id:     $REQUEST_ID"
  echo "  user_id:        $BANK_USER_ID"
  echo "  wallet_address: $WALLET_ADDRESS"
  echo "  status:         CREDENTIAL_REQUESTED"

  echo ""
  echo "--- Actor: CENTRAL BANK GOVERNANCE ---"
  echo "=== [3/7] Central Bank governance login ==="
  cb_governance_login
  echo "  CB_TOKEN: ${CB_TOKEN:0:60}..."

  echo ""
  echo "=== [4/7] Phase 2 — KYC Approval ==="
  approve_kyc
  echo "  status:    KYC_APPROVED"
  echo "  pop_nonce: ${POP_NONCE:0:32}..."

  echo ""
  echo "--- Actor: BANK-D OPERATOR ---"
  echo "=== [5/7] Phase 2.5 — Polling (via Bank-D proxy) ==="
  poll_onboarding_status
  echo "  Confirmed: status=KYC_APPROVED, pop_nonce present"

  echo ""
  echo "=== [6/7] Phase 3 — Complete Onboarding (via Bank-D proxy) ==="
  echo "  Submitting to /onboarding/complete (PoP signing handled by backend)..."
  complete_onboarding
  echo "  client_secret: $CLIENT_SECRET"
  echo "  tx_hash:       ${TX_HASH:-"(noop)"}"
  echo "  status:        ACTIVE"

  echo ""
  echo "--- Actor: COMMERCIAL BANK PARTICIPANT ---"
  echo "=== [7/7] PKI Login — Validation ==="
  if [ -n "$BANK_TOKEN" ]; then
    echo "  PKI login succeeded (chained in complete onboarding)"
    echo "  BANK_TOKEN: ${BANK_TOKEN:0:60}..."
  else
    echo "  WARNING: PKI login was not completed during onboarding."
    echo "  pki_login_error: $PKI_LOGIN_ERROR"
  fi

  echo ""
  echo "======================================================"
  echo "  Onboarding completed successfully"
  echo "======================================================"
  echo ""
  echo "  BANK_USER_ID     : $BANK_USER_ID"
  echo "  WALLET_ADDRESS   : $WALLET_ADDRESS"
  echo "  TX_HASH          : ${TX_HASH:-"(noop)"}"
  echo "  CLIENT_SECRET    : $CLIENT_SECRET"
  echo ""
  echo "  Tokens (truncated):"
  echo "    BANK_OPERATOR_TOKEN: ${BANK_OPERATOR_TOKEN:0:60}..."
  echo "    CB_TOKEN           : ${CB_TOKEN:0:60}..."
  echo "    BANK_TOKEN (PKI)   : ${BANK_TOKEN:0:60}..."
  echo ""
  echo "  >>> Save CLIENT_SECRET — shown only once. <<<"
  echo ""
}

main "$@"
