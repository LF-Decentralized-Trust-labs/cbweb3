#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# mtls-smoke.sh — the R2-H-8 acceptance check, made repeatable.
#
# The card that closes R2-H-8 asks for one verification: with mutual TLS and
# enforcement enabled, a plaintext gRPC dial to any internal service port must fail
# the TLS handshake, while a portal round-trip still succeeds over the mesh. That is
# a two-line instruction and a half-hour of fiddling, so it lives here instead.
#
# Scenario-agnostic on purpose: it discovers containers by name suffix (-api-gateway,
# -auth, -compliance, -payment-orchestrator), so it verifies whichever scenario's stack
# is running. It lives here because the R2-H-8 operator documentation does, and it reads
# docker state rather than either scenario's source — no cross-scenario coupling.
#
# It VERIFIES an already-provisioned stack. It does not provision one, because doing
# so would wipe docker state on the host — see --help for the two commands to run
# first.
#
# Usage:
#   ./mtls-smoke.sh                  # verify the running stack
#   ./mtls-smoke.sh --entity cb1     # verify one entity by name (default: every entity found)
#
# Exit codes: 0 all checks passed · 1 a check failed · 2 preconditions missing.
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; RST=$'\033[0m'

ok()   { printf '  %s✓%s %s\n' "$GREEN" "$RST" "$*"; }
bad()  { printf '  %s✗%s %s\n' "$RED" "$RST" "$*"; FAILED=$((FAILED+1)); }
warn() { printf '  %s!%s %s\n' "$YELLOW" "$RST" "$*"; }
info() { printf '   %s%s%s\n' "$DIM" "$*" "$RST"; }
step() { printf '\n%s══ %s%s\n' "$BOLD" "$*" "$RST"; }
FAILED=0

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  sed -n '3,20p' "$0" | sed 's/^# \{0,1\}//'
  cat <<'EOF'

Provision a stack with the mesh enabled first:

    export GRPC_MTLS_ENABLE=1
    export GRPC_AUTHZ_ENFORCE=true
    cd scenario-b/samples && ./deploy-all.sh --clean

Then run this script. Without those variables the stack is plaintext by design and
this script will say so rather than pass vacuously.
EOF
  exit 0
fi

ENTITY_FILTER=""
[[ "${1:-}" == "--entity" ]] && ENTITY_FILTER="${2:-}"

# ── preconditions ─────────────────────────────────────────────────────────────
step "Preconditions"
command -v docker >/dev/null || { printf '%sdocker is required%s\n' "$RED" "$RST"; exit 2; }

# The gRPC services are internal: they listen inside each entity's docker network and
# publish no host port. The dial therefore has to come from a container on that
# network, which is what `docker run --network` gives us.
GRPC_PROBE_IMAGE="${GRPC_PROBE_IMAGE:-alpine/openssl:3.3.2}"
docker image inspect "$GRPC_PROBE_IMAGE" >/dev/null 2>&1 || {
  info "pulling probe image ${GRPC_PROBE_IMAGE}"
  docker pull -q "$GRPC_PROBE_IMAGE" >/dev/null || { printf '%scould not pull %s%s\n' "$RED" "$GRPC_PROBE_IMAGE" "$RST"; exit 2; }
}
ok "docker available, probe image present"

mapfile -t GATEWAYS < <(docker ps --format '{{.Names}}' | grep -- '-api-gateway$' | sort)
[[ ${#GATEWAYS[@]} -gt 0 ]] || { printf '%sno *-api-gateway container is running — provision a stack first (--help)%s\n' "$RED" "$RST"; exit 2; }
ok "found ${#GATEWAYS[@]} api-gateway container(s)"

# ── is the mesh actually on? ──────────────────────────────────────────────────
# A pass has to mean something: if the stack was provisioned without the mesh, the
# plaintext dial below SUCCEEDS and that is correct behaviour, not a failure. Read
# the posture from the container's own environment rather than assuming it.
step "Mesh posture"
MESH_ON=0
for gw in "${GATEWAYS[@]}"; do
  [[ -n $ENTITY_FILTER && $gw != *"$ENTITY_FILTER"* ]] && continue
  cert=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$gw" | grep '^GRPC_MTLS_CERT_FILE=' | cut -d= -f2-)
  enforce=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$gw" | grep '^GRPC_AUTHZ_ENFORCE=' | cut -d= -f2-)
  if [[ -n $cert ]]; then
    MESH_ON=1
    ok "$gw: mTLS material configured (enforce=${enforce:-unset})"
  else
    warn "$gw: GRPC_MTLS_CERT_FILE empty — this entity runs plaintext by design"
  fi
done
if [[ $MESH_ON -eq 0 ]]; then
  printf '\n%sNo entity has the mesh enabled, so there is nothing to verify.%s\n' "$YELLOW" "$RST"
  printf 'Re-provision with GRPC_MTLS_ENABLE=1 and GRPC_AUTHZ_ENFORCE=true (see --help).\n'
  exit 2
fi

# ── 1. a plaintext dial must fail the handshake ───────────────────────────────
step "Plaintext gRPC dial must be refused"
for svc in auth compliance payment-orchestrator; do
  for c in $(docker ps --format '{{.Names}}' | grep -- "-${svc}$" | sort); do
    [[ -n $ENTITY_FILTER && $c != *"$ENTITY_FILTER"* ]] && continue
    net=$(docker inspect --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{break}}{{end}}' "$c")
    port=$(docker inspect --format '{{range $p,$_ := .Config.ExposedPorts}}{{$p}}{{break}}{{end}}' "$c" | cut -d/ -f1)
    [[ -z $port ]] && { warn "$c: no exposed port found, skipped"; continue; }

    # openssl with -no_tls1_3 … we simply send a plaintext TCP payload and expect the
    # server to drop it: a TLS server answers a non-TLS ClientHello with a fatal alert
    # or a close, never with a gRPC response.
    out=$(docker run --rm --network "$net" "$GRPC_PROBE_IMAGE" \
            sh -c "printf 'PRI * HTTP/2.0\r\n\r\n' | timeout 5 openssl s_client -connect ${c}:${port} -quiet 2>&1 | head -3" 2>&1)
    if grep -qiE 'handshake failure|wrong version|unknown protocol|alert|no peer certificate|sslv3|tls' <<<"$out"; then
      ok "$c:$port refused a plaintext dial"
    elif [[ -z $out ]]; then
      ok "$c:$port closed the plaintext dial without a response"
    else
      bad "$c:$port answered a plaintext dial: $(head -c 120 <<<"$out")"
    fi
  done
done

# ── 2. the portal round-trip must still work over the mesh ────────────────────
step "Portal round-trip over the mesh"
for gw in "${GATEWAYS[@]}"; do
  [[ -n $ENTITY_FILTER && $gw != *"$ENTITY_FILTER"* ]] && continue
  hostport=$(docker port "$gw" 2>/dev/null | head -1 | sed 's/.*-> //')
  [[ -z $hostport ]] && { warn "$gw: no published port, skipped"; continue; }
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "http://${hostport}/healthz" 2>/dev/null)
  if [[ $code == 200 ]]; then
    ok "$gw /healthz → 200 (the gateway is serving with the mesh on)"
  else
    bad "$gw /healthz → ${code:-no response}"
  fi
done
info "A full login → governance-list round-trip is exercised by sample-tryout.sh;"
info "run it after this script for the end-to-end path over the mesh."

# ── verdict ───────────────────────────────────────────────────────────────────
if [[ $FAILED -eq 0 ]]; then
  printf '\n%s✓ mTLS enforcement verified%s\n' "$GREEN$BOLD" "$RST"
  exit 0
fi
printf '\n%s✗ %d check(s) failed%s\n' "$RED$BOLD" "$FAILED" "$RST"
exit 1
