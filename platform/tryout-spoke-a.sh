#!/usr/bin/env bash
# tryout-spoke-a.sh — Spoke-A onboarding / participant registration
#
# Modes:
#   full (default)   CB login; register one user per onboardable ROLE; then CSR+KYC+wallet/bind only for bank-001 (PKI)
#   register         Register one participant: register <ROLE> [username]
#   register-all     Register one user per onboardable role; optional --approve-kyc
#   help             This help text
#
# Compliance PENDING vs login:
#   - POST /auth/login (direct) does not block on PENDING; authenticates with Keycloak. clientId may be userId (UUID)
#     or Keycloak username (auth-service resolves UUID -> username).
#   - approve-kyc requires an on-chain wallet; roles without KMS (NOC, Supervisor, Governance Officer) typically have no wallet
#     -> approve-kyc calls may return 412 and the participant stays PENDING in the DB (expected; does not imply 401 on login).
#   - Treasury in full mode: this script does NOT submit CSR for Treasury; do not call approve-kyc for Treasury here
#     until you have CSR+cert like the PKI flow. Optional future: separate flag/flow after CSR per role.
#
# Roles for POST /compliance/register (ROLE_GOVERNANCE = Keycloak service account, not created here):
#   ROLE_COMMERCIAL_BANK, ROLE_TREASURY     -> PKI login: nonce + /auth/wallet/bind (need CSR + CB-issued cert)
#   ROLE_NOC, ROLE_SUPERVISOR, ROLE_GOVERNANCE_OFFICER -> direct login: clientId=userId + clientSecret -> JWT
#
# General prerequisites:
#   - Spoke-A stack running (e.g. make dev.up-spoke-a)
#   - curl, jq, openssl, xxd
#   - backend/config/.env.infra.central-bank with KC_CLIENT_SECRET
# Full mode prerequisite:
#   - backend/config/pki/bank-001.{csr,key} (make pki.gen-bank-a pki.gen-commercial-banks)
#
# Examples:
#   ./tryout-spoke-a.sh
#   ./tryout-spoke-a.sh full
#   ./tryout-spoke-a.sh register ROLE_TREASURY
#   ./tryout-spoke-a.sh register ROLE_NOC my-noc-user
#   ./tryout-spoke-a.sh register-all --approve-kyc

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BASE_URL="${BASE_URL:-http://localhost:18080/api/v1}"
PKI_DIR="${PKI_DIR:-backend/config/pki}"
ENV_FILE="${ENV_FILE:-backend/config/.env.infra.central-bank}"

# Globals set by helpers (full flow)
CB_TOKEN=""
BANK_USER_ID=""
CLIENT_SECRET=""
ISSUED_CERT_PEM=""
CSR_LAST_EXPIRES=""
BANK_TOKEN=""

# ---------------------------------------------------------------------------
# Usage
# ---------------------------------------------------------------------------

usage() {
  cat <<'EOF'
tryout-spoke-a.sh — Spoke-A onboarding / participant registration

Modes:
  full (default)   CB login; register all 5 roles; then CSR+KYC+PKI wallet/bind only for bank-001 (ROLE_COMMERCIAL_BANK)
  register         Register one participant: register <ROLE> [username]
  register-all     Register one user per onboardable role; optional --approve-kyc
  help             This help text

Roles for POST /compliance/register (ROLE_GOVERNANCE = Keycloak service account, not created here):
  ROLE_COMMERCIAL_BANK, ROLE_TREASURY     -> PKI login: nonce + /auth/wallet/bind (need CSR + CB-issued cert)
  ROLE_NOC, ROLE_SUPERVISOR, ROLE_GOVERNANCE_OFFICER -> direct login: clientId=userId + clientSecret -> JWT

General prerequisites:
  - Spoke-A stack running (e.g. make dev.up-spoke-a)
  - curl, jq, openssl, xxd
  - backend/config/.env.infra.central-bank with KC_CLIENT_SECRET
Full mode prerequisite:
  - backend/config/pki/bank-001.{csr,key} (make pki.gen-bank-a pki.gen-commercial-banks)

Examples:
  ./tryout-spoke-a.sh
  ./tryout-spoke-a.sh full
  ./tryout-spoke-a.sh register ROLE_TREASURY
  ./tryout-spoke-a.sh register ROLE_NOC my-noc-user
  ./tryout-spoke-a.sh register-all --approve-kyc

Usage:
  tryout-spoke-a.sh [help|-h|--help]
  tryout-spoke-a.sh [full]
  tryout-spoke-a.sh register <ROLE> [username]
  tryout-spoke-a.sh register-all [--approve-kyc]

Valid ROLE values for register / register-all:
  ROLE_COMMERCIAL_BANK  ROLE_TREASURY  ROLE_NOC  ROLE_SUPERVISOR  ROLE_GOVERNANCE_OFFICER

Optional environment variables: BASE_URL, PKI_DIR, ENV_FILE
EOF
}

# ---------------------------------------------------------------------------
# Dependency checks
# ---------------------------------------------------------------------------

require_cmds() {
  for cmd in curl jq openssl xxd; do
    if ! command -v "$cmd" &>/dev/null; then
      echo "ERROR: dependency '$cmd' not found. Install it before continuing." >&2
      exit 1
    fi
  done
}

require_env_file() {
  if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: environment file '$ENV_FILE' not found." >&2
    echo "      Run 'make dev.up' first." >&2
    exit 1
  fi
}

require_pki_bank001() {
  if [ ! -f "$PKI_DIR/bank-001.csr" ] || [ ! -f "$PKI_DIR/bank-001.key" ]; then
    echo "ERROR: PKI files not found at $PKI_DIR/bank-001.{csr,key}." >&2
    echo "      Run 'make pki.gen-bank-a pki.gen-commercial-banks' first." >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Auth / API helpers
# ---------------------------------------------------------------------------

cb_login() {
  require_env_file
  local KC_SECRET
  KC_SECRET=$(grep KC_CLIENT_SECRET "$ENV_FILE" | cut -d= -f2-)
  if [ -z "$KC_SECRET" ]; then
    echo "ERROR: KC_CLIENT_SECRET not found in $ENV_FILE." >&2
    echo "      Run './deploy/local/keycloak/get_credentials_direct.sh --entity central-bank' to sync it." >&2
    exit 1
  fi
  local CB_LOGIN_RESP
  CB_LOGIN_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"clientId\": \"central-bank-client\", \"clientSecret\": \"$KC_SECRET\"}")
  CB_TOKEN=$(echo "$CB_LOGIN_RESP" | jq -r '.accessToken // empty')
  if [ -z "$CB_TOKEN" ]; then
    echo "ERROR: Central Bank login failed." >&2
    echo "Response: $CB_LOGIN_RESP" >&2
    exit 1
  fi
}

# POST /compliance/register. Prints full JSON body to stdout.
# Returns: 0 if 201 and userId present, 2 if 409 conflict, 1 other failure
register_participant_http() {
  local username=$1 email=$2 role=$3 institution=$4 country=$5 bank_code=$6
  local tmp code body uid
  tmp=$(mktemp)
  code=$(curl -sS -X POST "$BASE_URL/compliance/register" \
    --cookie "access_token=$CB_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg u "$username" \
      --arg e "$email" \
      --arg r "$role" \
      --arg i "$institution" \
      --arg c "$country" \
      --arg b "$bank_code" \
      '{username: $u, email: $e, role: $r, institution_name: $i, country: $c, bank_code: $b}')" \
    -o "$tmp" -w "%{http_code}")
  body=$(cat "$tmp")
  rm -f "$tmp"
  echo "$body"
  uid=$(echo "$body" | jq -r '.userId // empty')
  if [ "$code" = "201" ] && [ -n "$uid" ]; then
    return 0
  fi
  if [ "$code" = "409" ]; then
    return 2
  fi
  return 1
}

submit_csr_for_user() {
  local user_id=$1 role=$2 institution_name=$3 csr_path=$4
  local CSR_PEM CSR_RESP
  CSR_PEM=$(cat "$csr_path")
  CSR_RESP=$(curl -s -X POST "$BASE_URL/governance/registry/csr" \
    --cookie "access_token=$CB_TOKEN" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg csr "$CSR_PEM" \
      --arg uid "$user_id" \
      --arg role "$role" \
      --arg name "$institution_name" \
      '{csr_pem: $csr, user_id: $uid, role: $role, institution_name: $name}')")
  ISSUED_CERT_PEM=$(echo "$CSR_RESP" | jq -r '.cert_pem // empty')
  CSR_LAST_EXPIRES=$(echo "$CSR_RESP" | jq -r '.expires_at // "N/A"')
  if [ -z "$ISSUED_CERT_PEM" ]; then
    echo "ERROR: CSR submission failed." >&2
    echo "Response: $CSR_RESP" >&2
    return 1
  fi
  return 0
}

approve_kyc_subject() {
  local subject=$1 reason=$2
  curl -s -X POST "$BASE_URL/compliance/approve-kyc" \
    --cookie "access_token=$CB_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg s "$subject" --arg r "$reason" '{subject: $s, reason: $r}')"
}

# PKI login steps 5-6: nonce + wallet/bind. Sets BANK_TOKEN.
pki_wallet_bind() {
  local user_id=$1 client_secret=$2 key_path=$3 cert_pem=$4
  local NONCE_RESP NONCE NONCE_SIG BIND_RESP
  NONCE_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    --data "$(jq -n --arg cid "$user_id" --arg sec "$client_secret" '{clientId: $cid, clientSecret: $sec}')")
  NONCE=$(echo "$NONCE_RESP" | jq -r '.nonce // empty')
  if [ -z "$NONCE" ]; then
    echo "ERROR: PKI login step 1 failed - no nonce." >&2
    echo "Response: $NONCE_RESP" >&2
    return 1
  fi
  NONCE_SIG=$(echo -n "$NONCE" | xxd -r -p \
    | openssl dgst -sha256 -sign "$key_path" \
    | xxd -p | tr -d '\n')
  BIND_RESP=$(curl -s -X POST "$BASE_URL/auth/wallet/bind" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg uid "$user_id" \
      --arg sig "$NONCE_SIG" \
      --arg cert "$cert_pem" \
      '{user_id: $uid, nonce_signature_hex: $sig, cert_pem: $cert}')")
  BANK_TOKEN=$(echo "$BIND_RESP" | jq -r '.accessToken // empty')
  if [ -z "$BANK_TOKEN" ]; then
    echo "ERROR: PKI login step 2 (wallet/bind) failed." >&2
    echo "Response: $BIND_RESP" >&2
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# Defaults per ROLE (register / register-all)
# ---------------------------------------------------------------------------

is_valid_onboard_role() {
  case "$1" in
    ROLE_COMMERCIAL_BANK|ROLE_TREASURY|ROLE_NOC|ROLE_SUPERVISOR|ROLE_GOVERNANCE_OFFICER) return 0 ;;
    *) return 1 ;;
  esac
}

default_username_for_role() {
  case "$1" in
    ROLE_COMMERCIAL_BANK) echo tryout-commercial ;;
    ROLE_TREASURY) echo tryout-treasury ;;
    ROLE_NOC) echo tryout-noc ;;
    ROLE_SUPERVISOR) echo tryout-supervisor ;;
    ROLE_GOVERNANCE_OFFICER) echo tryout-governance-officer ;;
    *) echo "" ;;
  esac
}

# Prints one line: email|institution|country|bank_code (use | so institution may contain spaces)
defaults_for_register() {
  local role=$1 username=$2
  local email inst country bank
  email="ops@${username}.tryout.local"
  country="BR"
  case "$role" in
    ROLE_COMMERCIAL_BANK)
      inst="Tryout Commercial Bank"
      bank="001"
      ;;
    ROLE_TREASURY)
      inst="Tryout Treasury"
      bank="260"
      ;;
    ROLE_NOC)
      inst="Tryout NOC"
      bank="000"
      ;;
    ROLE_SUPERVISOR)
      inst="Tryout Supervisor"
      bank="000"
      ;;
    ROLE_GOVERNANCE_OFFICER)
      inst="Tryout Governance Officer"
      bank="000"
      ;;
    *)
      echo "ERROR: unknown role $role" >&2
      return 1
      ;;
  esac
  printf '%s|%s|%s|%s\n' "$email" "$inst" "$country" "$bank"
}

# ---------------------------------------------------------------------------
# Mode: full (bank-001 end-to-end)
# ---------------------------------------------------------------------------

cmd_full() {
  require_cmds
  require_env_file
  require_pki_bank001

  echo ""
  echo "=== [1/6] Central Bank login (Spoke-A) ==="
  cb_login
  echo "CB_TOKEN obtained (cookie access_token): ${CB_TOKEN:0:60}..."

  echo ""
  echo "=== [2/6] Register one user per onboardable ROLE (5) ==="
  echo "    PKI demo (steps 3-6) uses only ROLE_COMMERCIAL_BANK + username bank-001 + PKI bank-001.*"
  local roles_full r u email inst country bank reg_out rc uid
  roles_full=(
    ROLE_COMMERCIAL_BANK
    ROLE_TREASURY
    ROLE_NOC
    ROLE_SUPERVISOR
    ROLE_GOVERNANCE_OFFICER
  )
  BANK_USER_ID=""
  CLIENT_SECRET=""
  for r in "${roles_full[@]}"; do
    if [ "$r" = "ROLE_COMMERCIAL_BANK" ]; then
      u="bank-001"
      email="ops@bank-001.com.br"
      inst="Bank 001 S.A."
      country="BR"
      bank="001"
    else
      u=$(default_username_for_role "$r")
      IFS='|' read -r email inst country bank < <(defaults_for_register "$r" "$u")
    fi
    echo ""
    echo "  --- $r -> username=$u ---"
    rc=0
    reg_out=$(register_participant_http "$u" "$email" "$r" "$inst" "$country" "$bank") || rc=$?
    if [ "$rc" = "0" ]; then
      uid=$(echo "$reg_out" | jq -r '.userId')
      echo "  OK (save clientSecret below — one-time):"
      echo "$reg_out" | jq .
      if [ "$r" = "ROLE_COMMERCIAL_BANK" ]; then
        BANK_USER_ID=$uid
        CLIENT_SECRET=$(echo "$reg_out" | jq -r '.clientSecret // empty')
      fi
    elif [ "$rc" = "2" ]; then
      echo "  SKIP (409): user '$u' already exists — remove user or pick another username for a clean full run."
      if [ "$r" = "ROLE_COMMERCIAL_BANK" ]; then
        echo "ERROR: bank-001 is required for the PKI steps; cannot continue without a fresh registration (clientSecret)." >&2
        exit 1
      fi
    else
      echo "  FAIL: $reg_out" >&2
      if [ "$r" = "ROLE_COMMERCIAL_BANK" ]; then
        exit 1
      fi
    fi
  done

  echo ""
  echo "  --- Notes: PENDING / approve-kyc ---"
  echo "  New participants stay PENDING in compliance until KYC approval when applicable."
  echo "  approve-kyc only makes sense with a wallet; NOC/Supervisor/Officer typically stay PENDING (412 if you approve without wallet)."
  echo "  Direct login: use userId (UUID) or username + clientSecret from the registration JSON."
  echo "  Treasury in this run: no automatic CSR — do not expect approve-kyc like bank-001 without extra PKI steps."
  echo ""

  if [ -z "$BANK_USER_ID" ] || [ -z "$CLIENT_SECRET" ]; then
    echo "ERROR: ROLE_COMMERCIAL_BANK (bank-001) must register successfully in this run to obtain userId + clientSecret for PKI." >&2
    exit 1
  fi
  echo ""
  echo "  BANK_USER_ID (commercial / PKI path):  $BANK_USER_ID"
  echo "  CLIENT_SECRET (bank-001 only): $CLIENT_SECRET"
  echo ""
  echo ">>> WARNING: save bank-001 CLIENT_SECRET — shown only once. <<<"
  echo ">>> Other roles: if newly created, their clientSecret was only in each OK line (JSON). Re-run 'register ROLE ...' to see or use Keycloak admin. <<<"

  echo ""
  echo "=== [3/6] Submit bank-001 CSR ==="
  submit_csr_for_user "$BANK_USER_ID" "ROLE_COMMERCIAL_BANK" "Bank 001 S.A." "$PKI_DIR/bank-001.csr"
  echo "Certificate issued for $BANK_USER_ID"
  echo "Valid until: $CSR_LAST_EXPIRES"

  echo ""
  echo "=== [4/6] Approve bank-001 KYC ==="
  local KYC_RESP KYC_STATUS
  KYC_RESP=$(approve_kyc_subject "$BANK_USER_ID" "KYC approved - tryout-spoke-a")
  KYC_STATUS=$(echo "$KYC_RESP" | jq -r '.status // empty')
  if [ -z "$KYC_STATUS" ]; then
    echo "WARNING: KYC approval may have failed." >&2
    echo "Response: $KYC_RESP" >&2
  fi
  echo "KYC status: ${KYC_STATUS:-"(no status in response)"}"

  echo ""
  echo "=== [5/6] bank-001 PKI login - step 1: nonce ==="
  echo "(nonce obtained inside wallet_bind)"

  echo ""
  echo "=== [6/6] bank-001 PKI login - step 2: wallet/bind ==="
  pki_wallet_bind "$BANK_USER_ID" "$CLIENT_SECRET" "$PKI_DIR/bank-001.key" "$ISSUED_CERT_PEM"

  echo ""
  echo "======================================================"
  echo "  Flow completed successfully"
  echo "======================================================"
  echo "  Step 2 created (or skipped 409): all 5 onboardable roles."
  echo "  PKI path completed for: ROLE_COMMERCIAL_BANK (bank-001) only."
  echo "  BANK_USER_ID  : $BANK_USER_ID"
  echo "  CLIENT_SECRET : $CLIENT_SECRET  (bank-001)"
  echo "  CB_TOKEN      : ${CB_TOKEN:0:60}...  (cookie: access_token)"
  echo "  BANK_TOKEN    : ${BANK_TOKEN:0:60}...  (cookie: access_token)"
  echo "======================================================"
  echo ""
  echo "  >>> Treasury/NOC/Supervisor/Governance Officer: use clientSecret from step 2 JSON or direct login. <<<"
  echo "  >>> PKI roles without CSR in this run: still need CSR+KYC+bind (e.g. bank-002 for Treasury). <<<"
  echo "  >>> Doc: docs-reference/step-by-step/auth-login-onboarding-step-by-step.md (UUID vs username, PENDING, 412). <<<"
}

# ---------------------------------------------------------------------------
# Mode: register <ROLE> [username]
# ---------------------------------------------------------------------------

cmd_register() {
  local role username email inst country bank rest
  require_cmds
  require_env_file

  role=${1:-}
  if [ -z "$role" ]; then
    echo "ERROR: missing ROLE. Example: $0 register ROLE_NOC" >&2
    usage >&2
    exit 1
  fi
  if ! is_valid_onboard_role "$role"; then
    echo "ERROR: invalid ROLE '$role'" >&2
    exit 1
  fi
  shift
  username=${1:-}
  if [ -z "$username" ]; then
    username=$(default_username_for_role "$role")
  fi
  IFS='|' read -r email inst country bank < <(defaults_for_register "$role" "$username")

  echo "=== register: role=$role username=$username ==="
  cb_login

  local reg_out rc
  rc=0
  reg_out=$(register_participant_http "$username" "$email" "$role" "$inst" "$country" "$bank") || rc=$?
  echo "$reg_out" | jq .
  if [ "$rc" = "0" ]; then
    echo ""
    echo "OK: userId=$(echo "$reg_out" | jq -r '.userId')"
    echo "clientSecret (save now): $(echo "$reg_out" | jq -r '.clientSecret // empty')"
    case "$role" in
      ROLE_COMMERCIAL_BANK|ROLE_TREASURY)
        echo ""
        echo "Next (PKI): submit CSR via POST /governance/registry/csr, approve-kyc, then login with nonce + POST /auth/wallet/bind."
        echo "See: docs-reference/step-by-step/auth-login-onboarding-step-by-step.md"
        ;;
      *)
        echo ""
        echo "Next (no PKI): POST /auth/login with clientId=<userId> and clientSecret above -> JWT directly (no /auth/wallet/bind)."
        ;;
    esac
  elif [ "$rc" = "2" ]; then
    echo "SKIP: 409 conflict (username may already exist)." >&2
    exit 2
  else
    echo "ERROR: registration failed." >&2
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Mode: register-all [--approve-kyc]
# ---------------------------------------------------------------------------

cmd_register_all() {
  local approve_kyc=false
  while [ $# -gt 0 ]; do
    case "$1" in
      --approve-kyc) approve_kyc=true; shift ;;
      *) echo "ERROR: unknown option: $1" >&2; exit 1 ;;
    esac
  done

  require_cmds
  require_env_file

  echo "=== register-all (approve_kyc=$approve_kyc) ==="
  cb_login

  local roles r u email inst country bank reg_out rc uid
  roles=(
    ROLE_COMMERCIAL_BANK
    ROLE_TREASURY
    ROLE_NOC
    ROLE_SUPERVISOR
    ROLE_GOVERNANCE_OFFICER
  )
  for r in "${roles[@]}"; do
    u=$(default_username_for_role "$r")
    IFS='|' read -r email inst country bank < <(defaults_for_register "$r" "$u")
    echo ""
    echo "--- Register $r as $u ---"
    rc=0
    reg_out=$(register_participant_http "$u" "$email" "$r" "$inst" "$country" "$bank") || rc=$?
    if [ "$rc" = "0" ]; then
      uid=$(echo "$reg_out" | jq -r '.userId')
      echo "OK userId=$uid"
      if [ "$approve_kyc" = true ]; then
        echo "Approving KYC for $uid ..."
        approve_kyc_subject "$uid" "tryout register-all approve-kyc" | jq . || true
      fi
    elif [ "$rc" = "2" ]; then
      echo "SKIP (409): $u already exists"
    else
      echo "FAIL: $reg_out" >&2
    fi
  done
  echo ""
  echo "=== register-all finished ==="
  echo "PKI roles (COMMERCIAL_BANK, TREASURY): still need CSR + cert from BC before wallet/bind."
  echo "Non-PKI roles: login with clientId=userId + clientSecret from registration response (re-run single register to see secret if first time)."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  local mode
  mode="${1:-full}"
  case "$mode" in
    help|-h|--help)
      usage
      exit 0
      ;;
    full)
      shift || true
      if [ $# -gt 0 ]; then
        echo "ERROR: unexpected arguments after 'full': $*" >&2
        exit 1
      fi
      cmd_full
      ;;
    register)
      shift
      cmd_register "$@"
      ;;
    register-all)
      shift
      cmd_register_all "$@"
      ;;
    *)
      echo "ERROR: unknown mode '$mode'" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
