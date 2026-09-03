# SPDX-License-Identifier: Apache-2.0
#
# Derive the live happy-path suite's environment from toolkit manifests.
#
# The suite (`TestFullHappyPath`) used to discover its topology and its secrets from
# the `deploy/local` bring-up: host ports hardcoded as defaults in
# livehappy_config_test.go, and six Keycloak secrets read out of
# `backend/config/.env.infra.<entity>`. That bring-up was removed (PR #158), and the
# toolkit deliberately does not write those files — it renders a per-entity env
# instead, because the fixed files only ever covered six reference entities
# (see toolkit/engine/orchestrator/entityenv.go).
#
# So this script derives every value from the manifests the toolkit was applied with.
# Nothing here is a copy of a secret: the values are read out of the same YAML the
# operator handed to `cbweb3 apply`. Point it at other manifests and it follows them.
#
# Usage:
#     source scenario-a/tests/integration/toolkit-env.sh          # samples default
#     eval "$(bash scenario-a/tests/integration/toolkit-env.sh)"  # or eval the exports
#
# Override the six manifests to run against a different topology:
#     CB_A_MANIFEST=... BANK_A_MANIFEST=... source toolkit-env.sh
#
# Requires: yq (the repo already depends on it — see docs/TOOLCHAIN.md).

set -euo pipefail

_te_here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
_te_samples="$(cd "${_te_here}/../../samples" && pwd)"

# ── Role assignment ──────────────────────────────────────────────────────────
# The suite needs six entities across two spokes, and the four commercial banks are
# NOT interchangeable — each plays a named part in the correspondent-banking path:
#
#   BANK_A       originator on spoke-a          (locks the origin leg)
#   BANK_C       financial correspondent, spoke-a (receives the origin leg)
#   BANK_D       custodian on spoke-b           (locks the counter leg)
#   BANK_B       beneficiary on spoke-b         (receives the counter leg)
#
# A script cannot infer which sample bank plays which part, so the assignment is
# declared here. Defaults map the `samples/deploy-all.sh` topology: Brazil is
# spoke-a, Colombia is spoke-b.
CB_A_MANIFEST="${CB_A_MANIFEST:-${_te_samples}/brazil/central-bank-brazil.yaml}"
CB_B_MANIFEST="${CB_B_MANIFEST:-${_te_samples}/colombia/central-bank-colombia.yaml}"
BANK_A_MANIFEST="${BANK_A_MANIFEST:-${_te_samples}/brazil/bank-itau.yaml}"
BANK_C_MANIFEST="${BANK_C_MANIFEST:-${_te_samples}/brazil/bank-bradesco.yaml}"
BANK_D_MANIFEST="${BANK_D_MANIFEST:-${_te_samples}/colombia/bank-bancolombia.yaml}"
BANK_B_MANIFEST="${BANK_B_MANIFEST:-${_te_samples}/colombia/bank-davivienda.yaml}"

# ── Port derivation ──────────────────────────────────────────────────────────
# Every host port an entity publishes is derived from its Besu RPC port, in +1000
# bands (toolkit/engine/orchestrator/ports.go). The api-gateway band is +10000.
# Mirroring the offset here rather than reading a rendered file keeps this working
# before the stack is up — which is when the suite needs it.
_TE_PORT_OFFSET_API_GATEWAY=10000

_te_yq() { yq -r "$1" "$2"; }

_te_besu_port() { _te_yq '.spec.node.rpc.port' "$1"; }
_te_api_port()  { echo $(( $(_te_besu_port "$1") + _TE_PORT_OFFSET_API_GATEWAY )); }
_te_api_url()   { echo "http://localhost:$(_te_api_port "$1")"; }
_te_spoke_id()  { _te_yq '.spec.spoke.id' "$1"; }
_te_bank_id()   { _te_yq '.spec.bankId' "$1"; }

# _te_user / _te_pass read the operator account for a role out of spec.adminUsers.
# These are the accounts the toolkit provisions in Keycloak, and they are what the
# login endpoint wants: the `clientId`/`clientSecret` JSON fields on
# POST /api/v1/auth/login are the wire contract's names, not a description of the
# credential — since f55ade5b the auth service accepts only the OIDC password grant,
# so what belongs in them is a username and that user's password. Passing a realm
# client id/secret there is refused on purpose, which is why the suite returned 401
# against the old bring-up (see ../e2e/README.md).
_te_user() { _te_yq ".spec.adminUsers[] | select(.role==\"$2\") | .username" "$1" | head -1; }
_te_pass() { _te_yq ".spec.adminUsers[] | select(.role==\"$2\") | .password" "$1" | head -1; }

# _te_identity / _te_cb_identity build the Paladin signing identity for an entity's
# funded operator. Format mirrors toolkit/engine/orchestrator/pente.go
# (`funded_operator@<node>`) and paladin_registry.go (bank node = "<spokeID>-<bankID>",
# CB node = "<spokeID>-cb"). These are the strings the api-gateway checks against the
# Pente roster before proposing an FX agreement, and the members the orchestrator
# resolves a bilateral Pente group by — so a bank code will not do.
_te_identity()    { echo "funded_operator@$(_te_spoke_id "$1")-$(_te_bank_id "$1")"; }
_te_cb_identity() { echo "funded_operator@$(_te_spoke_id "$1")-cb"; }

_te_currency() { _te_yq '.spec.spoke.currency' "$1"; }

# _te_emit derives a DEFAULT: a value the caller already set wins and is echoed
# back unchanged. That is what lets the make target source this unconditionally —
# `make scenario-a.test-integration API_GW_BANK_A_URL=...` still overrides, and a
# run against a hand-built topology only has to name the values that differ.
# Output is `export NAME=<quoted>` lines, quoted with printf %q so a value containing
# a space, quote or $ survives being eval'd by a POSIX shell. That matters because the
# make recipe runs under /bin/sh (dash) and evals this script's output rather than
# sourcing it — dash rejects `set -o pipefail`, so the script itself must run in bash.
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
_te_emit API_GW_BANK_C_URL         "$(_te_api_url "$BANK_C_MANIFEST")"
_te_emit API_GW_BANK_D_URL         "$(_te_api_url "$BANK_D_MANIFEST")"
_te_emit API_GW_CENTRAL_BANK_A_URL "$(_te_api_url "$CB_A_MANIFEST")"
_te_emit API_GW_CENTRAL_BANK_B_URL "$(_te_api_url "$CB_B_MANIFEST")"

# ── Besu RPC (the always-up central-bank node on each spoke) ─────────────────
_te_emit BESU_SPOKE_A_RPC "http://localhost:$(_te_besu_port "$CB_A_MANIFEST")"
_te_emit BESU_SPOKE_B_RPC "http://localhost:$(_te_besu_port "$CB_B_MANIFEST")"

# ── Operator logins ──────────────────────────────────────────────────────────
_te_emit KC_BANK_A_CLIENT "$(_te_user "$BANK_A_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_A_SECRET "$(_te_pass "$BANK_A_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_B_CLIENT "$(_te_user "$BANK_B_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_B_SECRET "$(_te_pass "$BANK_B_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_C_CLIENT "$(_te_user "$BANK_C_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_C_SECRET "$(_te_pass "$BANK_C_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_D_CLIENT "$(_te_user "$BANK_D_MANIFEST" ROLE_BANK)"
_te_emit KC_BANK_D_SECRET "$(_te_pass "$BANK_D_MANIFEST" ROLE_BANK)"

_te_emit KC_CENTRAL_BANK_A_CLIENT "$(_te_user "$CB_A_MANIFEST" ROLE_GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_A_SECRET "$(_te_pass "$CB_A_MANIFEST" ROLE_GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_B_CLIENT "$(_te_user "$CB_B_MANIFEST" ROLE_GOVERNANCE)"
_te_emit KC_CENTRAL_BANK_B_SECRET "$(_te_pass "$CB_B_MANIFEST" ROLE_GOVERNANCE)"

# Minting needs ROLE_TREASURY; the governance operator above cannot mint.
_te_emit KC_CENTRAL_BANK_A_TREASURY_CLIENT "$(_te_user "$CB_A_MANIFEST" ROLE_TREASURY)"
_te_emit KC_CENTRAL_BANK_A_TREASURY_SECRET "$(_te_pass "$CB_A_MANIFEST" ROLE_TREASURY)"
_te_emit KC_CENTRAL_BANK_B_TREASURY_CLIENT "$(_te_user "$CB_B_MANIFEST" ROLE_TREASURY)"
_te_emit KC_CENTRAL_BANK_B_TREASURY_SECRET "$(_te_pass "$CB_B_MANIFEST" ROLE_TREASURY)"

# ── Paladin signing identities ───────────────────────────────────────────────
_te_emit IDENTITY_BANK_A          "$(_te_identity "$BANK_A_MANIFEST")"
_te_emit IDENTITY_CORRESPONDENT_A "$(_te_identity "$BANK_C_MANIFEST")"
_te_emit IDENTITY_CUSTODIAN       "$(_te_identity "$BANK_D_MANIFEST")"
_te_emit IDENTITY_BANK_B          "$(_te_identity "$BANK_B_MANIFEST")"
# The settlement agent is the SOURCE central bank, not the originating bank — see
# samples/sample-tryout.sh, which is the walkthrough this stack is built for.
_te_emit IDENTITY_SETTLEMENT_AGENT "$(_te_cb_identity "$CB_A_MANIFEST")"

# ── Spoke identity and currencies ────────────────────────────────────────────
# The FX legs are keyed by spoke id, and the currencies are each spoke's own. The
# suite used to hardcode spoke-a/spoke-b and USD/BRL, which no toolkit spoke has.
_te_emit SPOKE_A_ID "$(_te_spoke_id "$CB_A_MANIFEST")"
_te_emit SPOKE_B_ID "$(_te_spoke_id "$CB_B_MANIFEST")"
_te_emit FX_ORIGIN_CURRENCY  "$(_te_currency "$CB_A_MANIFEST")"
_te_emit FX_COUNTER_CURRENCY "$(_te_currency "$CB_B_MANIFEST")"

# ── Lifecycle ────────────────────────────────────────────────────────────────
# The toolkit is the only provisioning path and it is driven from samples/, never
# from the suite. Both are pinned so the suite can neither create nor destroy a stack.
_te_emit SKIP_UP   "1"
_te_emit SKIP_DOWN "1"
