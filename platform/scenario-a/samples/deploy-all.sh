#!/usr/bin/env bash
#
# deploy-all.sh — bring up the default Scenario A sample stack via the cbweb3 CLI.
#
# Deploys two spokes and their commercial banks, end to end:
#   • Brazil     (spoke-brl):        central-bank-brazil     + bank-itau, bank-bradesco
#   • Costa Rica (spoke-costa-rica): central-bank-costa-rica + cb1, cb2
#
# The two spokes use the two DIFFERENT live naming conventions on purpose:
#
#     spoke-brl-bank-itau        spokeId=spoke-brl         bankId=bank-itau
#     spoke-costa-rica-cb1       spokeId=spoke-costa-rica  bankId=cb1
#
# Code that recovers a bank or spoke id by splitting a node name on "-" is right
# for the first and wrong for the second. Four defects of that one root cause
# reached a deployed environment because every sample spoke used to have a
# two-segment id, so no local bring-up could reproduce the second shape
# (PRs #210, #211, #212, #213). Costa Rica replaced Colombia here rather than
# being added to it, so the default bring-up covers the class at the same cost.
# Colombia is still available in deploy-three.sh.
#
# Each central bank FOUNDS its spoke (Besu+Paladin, contracts, relay registration,
# operational backend + frontend portals) and emits a join bundle; each commercial
# bank JOINS as a full node and brings up its own operational stack.
#
# Idempotent: re-running resumes from the first incomplete step per entity.
#
# JWT validation (R2-H-6): the api-gateway auth service enforces both the token
# issuer (iss, derived from each entity's Keycloak URL + realm) and audience
# (aud). The toolkit provisions an oidc-audience-mapper on every backend login
# client so tokens carry aud=cbweb3-backend, and wires KEYCLOAK_AUDIENCE to match
# — nothing to set here. NOC backends run NOC_SKIP_AUTH=true (unchanged).
#
# Usage:
#   ./deploy-all.sh              # build CLI, start relay, deploy everything
#   ./deploy-all.sh --clean      # wipe docker (containers+volumes) + data dirs first
#   CBWEB3_BIN=/path/to/cbweb3 ./deploy-all.sh   # reuse a prebuilt CLI binary
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # .../scenario-a/samples
SCENARIO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"               # .../scenario-a
export CBWEB3_HOME="${SCENARIO_DIR}"
# All entities share ONE Docker host here, so their per-entity container-alias
# advertisedHosts are not externally routable: force the container-name Paladin
# transport + derived gRPC ports so they do not collide on the fixed peer port 9000.
export CBWEB3_SINGLE_HOST=1

BUNDLES_DIR="${SCRIPT_DIR}/bundles"
# The toolkit emits bundles to <outputDir>/bundles, where outputDir defaults to the
# parent of the manifest's node.dataDir. The sample manifests use a relative
# node.dataDir (cbweb3-data/<entity>), which the CLI resolves against its CWD — so
# data and bundles land under ${PWD}/cbweb3-data here. Override with CBWEB3_OUTPUT_DIR.
DATA_ROOT="${CBWEB3_OUTPUT_DIR:-${PWD}/cbweb3-data}"
DATA_BUNDLES="${DATA_ROOT}/bundles"

log() { printf '\n\033[1;36m[deploy] %s\033[0m\n' "$*"; }

# --- optional clean -----------------------------------------------------------
if [[ "${1:-}" == "--clean" ]]; then
  log "cleaning docker (containers + volumes) and data dirs…"
  docker rm -f $(docker ps -aq) 2>/dev/null || true
  docker volume rm $(docker volume ls -q) 2>/dev/null || true
  # Data lives under the user-owned ${DATA_ROOT} (no privileged /opt path, no sudo).
  rm -rf "${DATA_ROOT}" 2>/dev/null || true
fi

# --- CLI binary ---------------------------------------------------------------
BIN="${CBWEB3_BIN:-}"
if [[ -z "${BIN}" ]]; then
  BIN="${SCRIPT_DIR}/.cbweb3"
  log "building cbweb3 CLI → ${BIN}"
  ( cd "${SCENARIO_DIR}/toolkit" && go build -o "${BIN}" ./cmd/cbweb3 )
fi

# --- contract artifacts -------------------------------------------------------
# contracts/out/ is gitignored and NO apply step rebuilds it: the engine deploys
# whatever artifact is already on disk (deps.go points at out/<file>.sol/<name>.json).
# A stale out/ therefore deploys old bytecode while the engine calls the current
# ABI — e.g. an artifact predating IdentityRegistry's two-step onboarding has no
# verifyParticipant selector, so onboard-registry dies with "verifyParticipant tx
# reverted". deploy-lnet never hits this because ship.sh wipes the remote tree
# (out/ always empty → it compiles); a long-lived local checkout does. forge build
# is incremental, so this is cheap on repeat runs.
log "building scenario-a contract artifacts (forge build)…"
( cd "${SCENARIO_DIR}/contracts" \
    && { [[ -d dependencies ]] || forge soldeer install; } \
    && forge build >/dev/null )

# field <json-line> <key> — extract a string value from a flat JSON log line.
field() { printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p"; }

apply() { # <label> <manifest>
  log "$1"
  # The engine streams its step lifecycle as structured JSON to stdout in real time
  # (Go's stdout is unbuffered), then prints the final report at the end. Humanize the
  # lifecycle events as they arrive so the terminal shows live progress, and pass the
  # report (and anything unrecognized) through verbatim. Do NOT swallow output here:
  # a previous version grep-filtered stdout, hiding both progress and errors.
  local rc
  set +e
  "${BIN}" apply -f "$2" -o yaml | while IFS= read -r line; do
    case "$line" in
      *'"action":"step_started"'*)   printf '    -> %s\n'      "$(field "$line" step)" ;;
      *'"action":"step_completed"'*) printf '    [ok]   %s\n'  "$(field "$line" step)" ;;
      *'"action":"step_skipped"'*)   printf '    [skip] %s\n'  "$(field "$line" step)" ;;
      *'"action":"step_detail"'*)    printf '       .. %s\n'   "$(field "$line" detail)" ;;
      *'"action":"step_failed"'*)    printf '    [FAIL] %s: %s\n' "$(field "$line" step)" "$(field "$line" error)" ;;
      *) printf '%s\n' "$line" ;;
    esac
  done
  rc=${PIPESTATUS[0]}
  set -e
  if [[ "${rc}" -ne 0 ]]; then
    log "apply failed for $(basename "$2") (exit ${rc}) — see the report above; aborting."
    exit "${rc}"
  fi
}

copy_bundle() { # <spoke-id>
  # The joining banks reference ../bundles/<spoke>.bundle.yaml (i.e. samples/bundles),
  # which is gitignored and therefore absent on a clean checkout — create it first.
  mkdir -p "${BUNDLES_DIR}"
  cp "${DATA_BUNDLES}/$1.bundle.yaml" "${BUNDLES_DIR}/"
  log "bundle $1 → samples/bundles/"
}

# --- relay (hard prerequisite of the register-relay step) ---------------------
log "starting relay (cacti)…"
bash "${SCENARIO_DIR}/provisioning/scripts/start-cacti.sh" >/dev/null
log "relay healthy at http://localhost:4000"

# --- Brazil spoke -------------------------------------------------------------
apply "Brazil — found central-bank-brazil (spoke-brl)" "${SCRIPT_DIR}/brazil/central-bank-brazil.yaml"
copy_bundle spoke-brl
apply "Brazil — join bank-itau"     "${SCRIPT_DIR}/brazil/bank-itau.yaml"
apply "Brazil — join bank-bradesco" "${SCRIPT_DIR}/brazil/bank-bradesco.yaml"

# --- Costa Rica spoke (the LNET convention: hyphenated id + bare bank ids) ----
# Both founding central banks default their Paladin to host port 31648; on a single
# host the second spoke must use a distinct port. (Besu/backend/frontend ports are
# already derived per-entity from each manifest's Besu RPC port.)
export CBWEB3_PALADIN_CB_URL="http://localhost:31748"
apply "Costa Rica — found central-bank-costa-rica (spoke-costa-rica)" "${SCRIPT_DIR}/costa-rica/central-bank-costa-rica.yaml"
copy_bundle spoke-costa-rica
apply "Costa Rica — join cb1" "${SCRIPT_DIR}/costa-rica/cb1.yaml"
apply "Costa Rica — join cb2" "${SCRIPT_DIR}/costa-rica/cb2.yaml"

# --- summary ------------------------------------------------------------------
log "all samples deployed. Endpoints (api-gateway /healthz, portals on /):"
cat <<'EOF'

  Brazil (spoke-brl)
    central-bank   api http://localhost:18645   governance http://localhost:25645
                                                 treasury   http://localhost:26645
                                                 supervisor http://localhost:30645
                                                 noc        http://localhost:32645
                                                 launcher   http://localhost:5191
    bank-itau      api http://localhost:18646   portal     http://localhost:25646
                                                 launcher   http://localhost:5192
    bank-bradesco  api http://localhost:18647   portal     http://localhost:25647
                                                 launcher   http://localhost:5193

  Costa Rica (spoke-costa-rica) — LNET convention: hyphenated spoke id, bare bank ids
    central-bank   api http://localhost:18685   governance http://localhost:25685
                                                 treasury   http://localhost:26685
                                                 supervisor http://localhost:30685
                                                 noc        http://localhost:32685
                                                 launcher   http://localhost:5200
    cb1            api http://localhost:18686   portal     http://localhost:25686
                                                 launcher   http://localhost:5201
    cb2            api http://localhost:18687   portal     http://localhost:25687
                                                 launcher   http://localhost:5202

  The identities below are the ones a positional split gets wrong — use them when
  exercising anything that maps a Paladin identity to a bank or spoke id:

    funded_operator@spoke-costa-rica-cb   spokeId spoke-costa-rica  (a CB node)
    funded_operator@spoke-costa-rica-cb1  bankId  cb1  (a split reads "rica-cb1")
    funded_operator@spoke-costa-rica-cb2  bankId  cb2  (a split reads "rica-cb2")

  The launcher (per entity) is the A/B entry point; it lists that entity's Scenario A
  and B portals. Build the image once: ( cd ../../launcher && ./build.sh ).

  Note: a joining bank completes onboarding via its Governance Portal. On KYC
  approval the CB compliance service both issues the CB-signed certificate and
  whitelists the bank's wallet in the on-chain IdentityRegistry (registerParticipant,
  onlyRole GOVERNANCE_ROLE), signed by the CB governance key (CB_PRIVATE_KEY). No
  manual CLI step is required. Bilateral Pente/FXAgreement remains a governance-gated
  step in the join's soft tail (see toolkit/E2E-STATUS.md).
EOF
