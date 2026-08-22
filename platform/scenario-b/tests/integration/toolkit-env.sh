# SPDX-License-Identifier: Apache-2.0
#
# Derive the Scenario B happy-path suite's environment from toolkit manifests.
#
# Same problem, same shape, as scenario-a/tests/integration/toolkit-env.sh — read that
# file's header for the reasoning. In short: the suite used to take its topology from
# the `deploy/local` bring-up (host ports hardcoded as defaults) and its credentials
# from `backend/config/.env.infra.<entity>`. That bring-up was removed (PR #158) and the
# toolkit deliberately does not write those files, so both had to come from somewhere
# else. They come from the manifests the toolkit was applied with.
#
# Usage:
#     source scenario-b/tests/integration/toolkit-env.sh
#     eval "$(bash scenario-b/tests/integration/toolkit-env.sh)"
#
# Requires: yq.

set -euo pipefail

_te_here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
_te_samples="$(cd "${_te_here}/../../samples" && pwd)"

# ── Role assignment ──────────────────────────────────────────────────────────
# The suite drives a two-spoke corridor through the neutral hub and needs four
# entities plus the hub. Which sample plays which part cannot be inferred, so it is
# declared here. Defaults map `samples/deploy-all.sh`: Brazil is spoke-a, Argentina
# is spoke-b, matching the BRL<->ARS corridor that walkthrough opens.
HUB_MANIFEST="${HUB_MANIFEST:-${_te_samples}/hub/hub-cbweb3.yaml}"
CB_A_MANIFEST="${CB_A_MANIFEST:-${_te_samples}/brazil/central-bank-brazil.yaml}"
CB_B_MANIFEST="${CB_B_MANIFEST:-${_te_samples}/argentina/central-bank-argentina.yaml}"
BANK_A_MANIFEST="${BANK_A_MANIFEST:-${_te_samples}/brazil/bank-itau.yaml}"
BANK_B_MANIFEST="${BANK_B_MANIFEST:-${_te_samples}/argentina/bank-galicia.yaml}"

# ── Port derivation ──────────────────────────────────────────────────────────
# Scenario B derives an entity's host ports from its Besu RPC port with a DIFFERENT
# offset than Scenario A: api-gateway is +8000 here (+10000 there), Keycloak +7000.
# See toolkit/engine/orchestrator/step_found_spoke.go — `c.RPCPort+8000`. Getting this
# from the other scenario's constant would point the suite at ports nothing serves.
_TE_PORT_OFFSET_API_GATEWAY=8000

_te_yq() { yq -r "$1" "$2"; }

_te_besu_port() { _te_yq '.spec.node.rpc.port' "$1"; }
_te_api_url()   { echo "http://localhost:$(( $(_te_besu_port "$1") + _TE_PORT_OFFSET_API_GATEWAY ))"; }
_te_besu_url()  { echo "http://localhost:$(_te_besu_port "$1")"; }

# Scenario B's manifests carry BARE role names (GOVERNANCE, BANK, TREASURY), not the
# ROLE_-prefixed form Scenario A uses. Selecting on the wrong spelling silently yields
# an empty credential, which reaches Keycloak and comes back as an opaque 401.
_te_user() { _te_yq ".spec.adminUsers[] | select(.role==\"$2\") | .username" "$1" | head -1; }
_te_pass() { _te_yq ".spec.adminUsers[] | select(.role==\"$2\") | .password" "$1" | head -1; }

# Derives a DEFAULT: a value the caller already set wins and is echoed back unchanged.
# Output is `export NAME=<printf %q>` so a POSIX shell can eval it safely — the make
# recipe runs under /bin/sh (dash), which rejects this script's `set -o pipefail`, so
# the script runs in bash and only its output crosses the boundary.
_te_emit() {
  local cur="${!1-}"
  [[ -n "$cur" ]] && { printf 'export %s=%q   # kept: set by caller\n' "$1" "$cur"; return; }
  printf 'export %s=%q\n' "$1" "$2"
  # shellcheck disable=SC2163  # deliberate: also apply to the caller when sourced.
  export "$1=$2"
}

# ── Gateways ─────────────────────────────────────────────────────────────────
_te_emit API_GW_BANK_A_URL         "$(_te_api_url "$BANK_A_MANIFEST")"
_te_emit API_GW_BANK_B_URL         "$(_te_api_url "$BANK_B_MANIFEST")"
_te_emit API_GW_CENTRAL_BANK_A_URL "$(_te_api_url "$CB_A_MANIFEST")"
_te_emit API_GW_CENTRAL_BANK_B_URL "$(_te_api_url "$CB_B_MANIFEST")"

# ── Besu RPC (hub + the always-up central-bank node on each spoke) ───────────
_te_emit BESU_HUB_RPC     "$(_te_besu_url "$HUB_MANIFEST")"
_te_emit BESU_SPOKE_A_RPC "$(_te_besu_url "$CB_A_MANIFEST")"
_te_emit BESU_SPOKE_B_RPC "$(_te_besu_url "$CB_B_MANIFEST")"

# ── Operator logins ──────────────────────────────────────────────────────────
# Despite the KC_*_CLIENT / KC_*_SECRET names, what belongs here is a Keycloak USERNAME
# and that user's PASSWORD. The login endpoint's clientId/clientSecret JSON fields are
# the wire contract's names, not the credential type: the auth service accepts only the
# OIDC password grant, and a realm client id/secret is refused on purpose.
_te_emit KC_BANK_A_CLIENT "$(_te_user "$BANK_A_MANIFEST" BANK)"
_te_emit KC_BANK_A_SECRET "$(_te_pass "$BANK_A_MANIFEST" BANK)"
_te_emit KC_BANK_B_CLIENT "$(_te_user "$BANK_B_MANIFEST" BANK)"
_te_emit KC_BANK_B_SECRET "$(_te_pass "$BANK_B_MANIFEST" BANK)"

_te_emit KC_CENTRAL_BANK_A_CLIENT "$(_te_user "$CB_A_MANIFEST" GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_A_SECRET "$(_te_pass "$CB_A_MANIFEST" GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_B_CLIENT "$(_te_user "$CB_B_MANIFEST" GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_B_SECRET "$(_te_pass "$CB_B_MANIFEST" GOVERNANCE)"

# ── Bank codes ───────────────────────────────────────────────────────────────
# The participant registry keys banks by their manifest bankId. The suite hardcoded
# "bank-a"/"bank-b", so the bridge-out leg looked up a beneficiary that the spoke's
# registry has never heard of and answered BENEFICIARY_NOT_FOUND.
_te_bank_id() { _te_yq '.spec.bankId' "$1"; }

_te_emit BANK_A_CODE "$(_te_bank_id "$BANK_A_MANIFEST")"
_te_emit BANK_B_CODE "$(_te_bank_id "$BANK_B_MANIFEST")"

# ── Corridor identity ────────────────────────────────────────────────────────
# The hub prefixes each sovereign currency with "W-" when it wraps it, and names a
# pool by BOTH wrapped sides: "W-BRL-W-ARS", not "W-BRL-ARS". The suite hardcoded the
# short form, which resolved to nothing — the swap failed with
# `resolve pair "W-BRL-ARS": not found among active pairs`.
_te_currency() { _te_yq '.spec.spoke.currency' "$1"; }

_te_cur_a="$(_te_currency "$CB_A_MANIFEST")"
_te_cur_b="$(_te_currency "$CB_B_MANIFEST")"
_te_emit SOURCE_CURRENCY "$_te_cur_a"
_te_emit TARGET_CURRENCY "$_te_cur_b"
_te_emit POOL_PAIR       "W-${_te_cur_a}-W-${_te_cur_b}"

# ── Lifecycle ────────────────────────────────────────────────────────────────
# The toolkit is the single provisioning path and it is driven from samples/, never
# from the suite. Both are pinned so the suite can neither create nor destroy a stack.
_te_emit SKIP_UP   "1"
_te_emit SKIP_DOWN "1"
