#!/usr/bin/env bash
# tryout-spoke-a.sh
#
# Full bank-001 onboarding flow on Spoke-A:
#   [1] Central Bank login (ROLE_GOVERNANCE)
#   [2] Register bank-001 in Keycloak via /compliance/register
#   [3] Submit bank-001 CSR via /governance/registry/csr
#   [4] Approve bank-001 KYC (required for PKI login to work)
#   [5] bank-001 PKI login — step 1: get nonce
#   [6] bank-001 PKI login — step 2: sign nonce and get token
#
# Prerequisites:
#   - Spoke-A stack running (make dev.up-spoke-a)
#   - curl, jq, openssl, xxd installed
#   - backend/config/.env.infra.spoke-a with KC_CLIENT_SECRET set
#   - backend/config/pki/bank-001.csr and bank-001.key available

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

BASE_URL="http://localhost:18080/api/v1"
PKI_DIR="backend/config/pki"
ENV_FILE="backend/config/.env.infra.spoke-a"

# ---------------------------------------------------------------------------
# Dependency checks
# ---------------------------------------------------------------------------

for cmd in curl jq openssl xxd; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: dependency '$cmd' not found. Install it before continuing." >&2
    exit 1
  fi
done

if [ ! -f "$ENV_FILE" ]; then
  echo "ERROR: environment file '$ENV_FILE' not found." >&2
  echo "      Run 'make dev.up-spoke-a' first." >&2
  exit 1
fi

if [ ! -f "$PKI_DIR/bank-001.csr" ] || [ ! -f "$PKI_DIR/bank-001.key" ]; then
  echo "ERROR: PKI files not found at $PKI_DIR/bank-001.{csr,key}." >&2
  echo "      Run 'make pki.gen-spoke-a pki.gen-commercial-banks' first." >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# [1] Central Bank login (Spoke-A)
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Authenticate the governance actor (Central Bank) and obtain an administrative Bearer token.
# - This token (CB_TOKEN) authorizes the onboarding steps: register, CSR, and approve-kyc.
# - Without this token, the next calls return 401/403.

echo ""
echo "=== [1/6] Central Bank login (Spoke-A) ==="

KC_SECRET=$(grep KC_CLIENT_SECRET "$ENV_FILE" | cut -d= -f2-)
if [ -z "$KC_SECRET" ]; then
  echo "ERROR: KC_CLIENT_SECRET not found in $ENV_FILE." >&2
  echo "      Run './deploy/local/keycloak/get_credentials_direct.sh --spoke a' to sync it." >&2
  exit 1
fi

CB_LOGIN_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"clientId\": \"cbweb3-spoke-a-client\", \"clientSecret\": \"$KC_SECRET\"}")

# FRONTEND (expected contract):
# request:  { clientId, clientSecret }
# response: { accessToken, tokenType, expiresIn }
# note: for ROLE_GOVERNANCE, /auth/login returns a token directly (it does not return nonce).

CB_TOKEN=$(echo "$CB_LOGIN_RESP" | jq -r '.accessToken // empty')
if [ -z "$CB_TOKEN" ]; then
  echo "ERROR: Central Bank login failed." >&2
  echo "Response: $CB_LOGIN_RESP" >&2
  exit 1
fi

echo "CB_TOKEN obtained: ${CB_TOKEN:0:60}..."

# ---------------------------------------------------------------------------
# [2] Register bank-001 in Keycloak
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Create the participant in Keycloak and capture the canonical ID (BANK_USER_ID / userId).
# - This userId is the "subject" used in CSR, KYC, and PKI login.
# - Without a valid userId, CSR/KYC/bind fail (e.g., 404 not found).

echo ""
echo "=== [2/6] Register bank-001 ==="

REGISTER_RESP=$(curl -s -X POST "$BASE_URL/compliance/register" \
  -H "Authorization: Bearer $CB_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username":         "bank-001",
    "email":            "ops@bank-001.com.br",
    "role":             "ROLE_COMMERCIAL_BANK",
    "institution_name": "Bank 001 S.A.",
    "country":          "BR",
    "bank_code":        "001"
  }')

# FRONTEND (expected contract):
# request:  username/email/role/institution_name/country/bank_code
# response: userId + status (typically PENDING) + clientSecret (ONE TIME) + registration metadata
# note: existing username conflicts usually return 409.
# SECURITY: clientSecret is returned ONLY at registration. Store it securely.

BANK_USER_ID=$(echo "$REGISTER_RESP" | jq -r '.userId // empty')
if [ -z "$BANK_USER_ID" ]; then
  echo "ERROR: bank-001 registration failed." >&2
  echo "Response: $REGISTER_RESP" >&2
  echo "Tip: if the error is 409, user 'bank-001' already exists in Keycloak." >&2
  exit 1
fi

CLIENT_SECRET=$(echo "$REGISTER_RESP" | jq -r '.clientSecret // empty')
if [ -z "$CLIENT_SECRET" ]; then
  echo "ERROR: clientSecret not returned in registration response." >&2
  echo "Response: $REGISTER_RESP" >&2
  exit 1
fi

echo "BANK_USER_ID:  $BANK_USER_ID"
echo "CLIENT_SECRET: $CLIENT_SECRET"
echo "Status:        $(echo "$REGISTER_RESP" | jq -r '.status // "N/A"')"
echo ""
echo ">>> WARNING: save the CLIENT_SECRET above. It will NOT be recoverable after this point. <<<"

# ---------------------------------------------------------------------------
# [3] Submit bank-001 CSR
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Send the participant CSR so governance can sign it via the local CA.
# - The response contains cert_pem, which will be used in the bind step (/auth/wallet/bind).
# - This step does NOT activate the participant; status remains PENDING until approve-kyc.

echo ""
echo "=== [3/6] Submit bank-001 CSR ==="

CSR_PEM=$(cat "$PKI_DIR/bank-001.csr")

CSR_RESP=$(curl -s -X POST "$BASE_URL/governance/registry/csr" \
  -H "Authorization: Bearer $CB_TOKEN" \
  -H "Content-Type: application/json" \
  --data "$(jq -n \
    --arg csr  "$CSR_PEM" \
    --arg uid  "$BANK_USER_ID" \
    --arg role "ROLE_COMMERCIAL_BANK" \
    --arg name "Bank 001 S.A." \
    '{csr_pem: $csr, user_id: $uid, role: $role, institution_name: $name}')")

# FRONTEND (expected contract):
# request:  { csr_pem, user_id, role, institution_name }
# response: { cert_pem, expires_at, ... }
# critical note: use safe JSON serialization (here via jq) to preserve PEM line breaks.
# if the PEM is badly serialized, the API usually returns 400.

BANK_CERT_PEM=$(echo "$CSR_RESP" | jq -r '.cert_pem // empty')
if [ -z "$BANK_CERT_PEM" ]; then
  echo "ERROR: CSR submission failed." >&2
  echo "Response: $CSR_RESP" >&2
  exit 1
fi

CERT_EXPIRES=$(echo "$CSR_RESP" | jq -r '.expires_at // "N/A"')
echo "Certificate issued for $BANK_USER_ID"
echo "Valid until: $CERT_EXPIRES"

# ---------------------------------------------------------------------------
# [4] Approve bank-001 KYC
# (prerequisite: without approval, PKI login returns PENDING and is rejected)
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Move the participant from PENDING to ACTIVE.
# - ACTIVE is a business prerequisite for commercial bank PKI login to work.

echo ""
echo "=== [4/6] Approve bank-001 KYC ==="

KYC_RESP=$(curl -s -X POST "$BASE_URL/compliance/approve-kyc" \
  -H "Authorization: Bearer $CB_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"subject\": \"$BANK_USER_ID\", \"reason\": \"KYC approved - tryout-spoke-a\"}")

# FRONTEND (expected contract):
# request:  { subject: userId, reason }
# response: { subject, status: "ACTIVE", ... }
# note: prerequisite errors (e.g., participant without wallet) may appear as 412.

KYC_STATUS=$(echo "$KYC_RESP" | jq -r '.status // empty')
if [ -z "$KYC_STATUS" ]; then
  echo "WARNING: KYC approval may have failed." >&2
  echo "Response: $KYC_RESP" >&2
fi

echo "KYC status: ${KYC_STATUS:-"(no status in response)"}"

# ---------------------------------------------------------------------------
# [5] bank-001 PKI login — step 1: get nonce
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Start the cryptographic challenge (challenge-response).
# - For PKI roles, /auth/login returns a temporary nonce (not JWT).
# - This nonce will be signed locally with the bank private key in the next step.

echo ""
echo "=== [5/6] bank-001 PKI login - step 1: get nonce ==="

# PKI roles (ROLE_COMMERCIAL_BANK) return { "nonce": "..." } instead of accessToken.
# clientSecret is the first factor: the server validates it before issuing the nonce.
NONCE_RESP=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  --data "$(jq -n \
    --arg cid "$BANK_USER_ID" \
    --arg sec "$CLIENT_SECRET" \
    '{clientId: $cid, clientSecret: $sec}')")

# FRONTEND (expected contract):
# request:  { clientId: userId, clientSecret: "<rawSecret from step 2>" }
# response: { nonce }
# note: nonce has a short TTL (5 min); if it expires, this step must be repeated.

NONCE=$(echo "$NONCE_RESP" | jq -r '.nonce // empty')
if [ -z "$NONCE" ]; then
  echo "ERROR: PKI login step 1 failed - 'nonce' field not returned." >&2
  echo "Response: $NONCE_RESP" >&2
  exit 1
fi

echo "Nonce obtained: $NONCE"

# ---------------------------------------------------------------------------
# [6] bank-001 PKI login — step 2: sign nonce and get token
# ---------------------------------------------------------------------------

# FRONTEND (step goal):
# - Prove possession of the private key corresponding to the certificate.
# - Send nonce signature + X.509 certificate for backend validation.
# - If validation succeeds, backend issues the bank session token (BANK_TOKEN).

echo ""
echo "=== [6/6] bank-001 PKI login - step 2: sign nonce and get token ==="

# Convert nonce from hex to bytes, sign with ECDSA P-256 + SHA-256 (DER), convert back to hex
NONCE_SIG=$(echo -n "$NONCE" | xxd -r -p \
  | openssl dgst -sha256 -sign "$PKI_DIR/bank-001.key" \
  | xxd -p | tr -d '\n')

# FRONTEND (equivalent in app/web):
# - convert nonce(hex) -> bytes
# - sign bytes with the client private key (algorithm expected by backend)
# - send signature in hexadecimal (nonce_signature_hex)

BIND_RESP=$(curl -s -X POST "$BASE_URL/auth/wallet/bind" \
  -H "Content-Type: application/json" \
  --data "$(jq -n \
    --arg uid  "$BANK_USER_ID" \
    --arg sig  "$NONCE_SIG" \
    --arg cert "$BANK_CERT_PEM" \
    '{user_id: $uid, nonce_signature_hex: $sig, cert_pem: $cert}')")

# FRONTEND (expected contract):
# request:  { user_id, nonce_signature_hex, cert_pem }
# response: { accessToken } (or access_token variation, depending on client adapter)
# common errors: 401 (invalid signature, expired nonce, invalid chain/certificate).

BANK_TOKEN=$(echo "$BIND_RESP" | jq -r '.accessToken // empty')
if [ -z "$BANK_TOKEN" ]; then
  echo "ERROR: PKI login step 2 failed." >&2
  echo "Response: $BIND_RESP" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
# Final summary
# ---------------------------------------------------------------------------

echo ""
echo "======================================================"
echo "  Flow completed successfully"
echo "======================================================"
echo "  BANK_USER_ID  : $BANK_USER_ID"
echo "  CLIENT_SECRET : $CLIENT_SECRET"
echo "  CB_TOKEN      : ${CB_TOKEN:0:60}..."
echo "  BANK_TOKEN    : ${BANK_TOKEN:0:60}..."
echo "======================================================"
echo ""
echo "  >>> Store the CLIENT_SECRET above. It is shown only once. <<<"
