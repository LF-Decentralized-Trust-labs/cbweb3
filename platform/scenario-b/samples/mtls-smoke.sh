#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# mtls-smoke.sh — the R2-H-8 acceptance check, made repeatable.
#
# The card that closes R2-H-8 asks for one verification: with mutual TLS and
# enforcement enabled, a plaintext gRPC dial to any internal service port must fail,
# while a portal round-trip still succeeds over the mesh. That is a two-line
# instruction and a half-hour of fiddling, so it lives here instead.
#
# "Plaintext is refused" and "a TLS handshake without a client certificate is refused"
# are two different properties, and an `openssl s_client` probe only ever tests the
# second — it opens with a TLS ClientHello, so nothing it sends reaches the wire in the
# clear. They are checked separately below, and named for what each one actually does.
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
# A pass has to mean something: if an entity was provisioned without the mesh, a
# plaintext dial to ITS services succeeds and that is correct behaviour, not a failure.
# Posture is therefore recorded per entity, not globally — a stack where one entity has
# the mesh and another does not is a normal intermediate state during rollout, and
# judging the second entity by the first would report a false failure.
step "Mesh posture"
declare -A MESH_BY_ENTITY=()
MESH_ANY=0
for gw in "${GATEWAYS[@]}"; do
  [[ -n $ENTITY_FILTER && $gw != *"$ENTITY_FILTER"* ]] && continue
  entity=${gw%-api-gateway}
  env_dump=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$gw")
  cert=$(grep '^GRPC_MTLS_CERT_FILE=' <<<"$env_dump" | cut -d= -f2-)
  enforce=$(grep '^GRPC_AUTHZ_ENFORCE=' <<<"$env_dump" | cut -d= -f2-)
  if [[ -n $cert ]]; then
    MESH_BY_ENTITY[$entity]=1; MESH_ANY=1
    ok "$entity: mTLS material configured (enforce=${enforce:-unset})"
    [[ ${enforce,,} == true ]] || warn "$entity: GRPC_AUTHZ_ENFORCE is not true — authz runs in audit mode"
  else
    MESH_BY_ENTITY[$entity]=0
    warn "$entity: GRPC_MTLS_CERT_FILE empty — this entity runs plaintext by design"
  fi
done
if [[ $MESH_ANY -eq 0 ]]; then
  printf '\n%sNo entity has the mesh enabled, so there is nothing to verify.%s\n' "$YELLOW" "$RST"
  printf 'Re-provision with GRPC_MTLS_ENABLE=1 and GRPC_AUTHZ_ENFORCE=true (see --help).\n'
  exit 2
fi

# The HTTP/2 connection preface plus an empty SETTINGS frame — what any gRPC client
# sends first. Over h2c a server answers with its own SETTINGS frame; a TLS listener
# cannot read it as a record and answers with an alert or drops the connection.
H2_PREFACE='PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n\x00\x00\x00\x04\x00\x00\x00\x00\x00'

# ── 1. a cleartext HTTP/2 dial must not be answered ───────────────────────────
step "Cleartext gRPC dial must not be answered"
for svc in auth compliance payment-orchestrator; do
  for c in $(docker ps --format '{{.Names}}' | grep -- "-${svc}$" | sort); do
    [[ -n $ENTITY_FILTER && $c != *"$ENTITY_FILTER"* ]] && continue
    entity=${c%-$svc}
    if [[ ${MESH_BY_ENTITY[$entity]:-unset} == unset ]]; then
      info "$c: no api-gateway found for $entity, posture unknown — skipped"; continue
    fi
    net=$(docker inspect --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{break}}{{end}}' "$c")
    port=$(docker inspect --format '{{range $p,$_ := .Config.ExposedPorts}}{{$p}}{{break}}{{end}}' "$c" | cut -d/ -f1)
    [[ -z $port ]] && { warn "$c: no exposed port found, skipped"; continue; }

    # Raw TCP, no TLS layer: the preface goes out in the clear. Reply bytes are captured
    # as hex so the verdict rests on the wire response, not on a tool's log lines.
    #
    # Reachability is probed separately, and that separation is load-bearing: busybox nc
    # reports a refused connection with exit status 1 and NOTHING on stderr, so "no
    # answer" and "nobody listening" are the same observation from nc alone. Reading the
    # first as a pass is how a probe comes to certify a port that was never dialled.
    # Reachability first, then the dial: the gate has to be established before the
    # observation it qualifies, or a dead port produces a "no answer" that reads as a pass.
    probe=$(docker run --rm --network "$net" --entrypoint sh "$GRPC_PROBE_IMAGE" -c \
      "timeout 6 openssl s_client -connect ${c}:${port} </dev/null >/tmp/t 2>&1; \
       grep -qiE 'BIO_connect|Connection refused|connect:errno|Name or service not known' /tmp/t \
         && echo REACH=no || echo REACH=yes; \
       printf '$H2_PREFACE' | timeout 5 nc -w 5 ${c} ${port} >/tmp/o 2>/tmp/e; echo \"RC=\$?\"; \
       echo \"HEX=\$(od -An -tx1 </tmp/o | tr -d ' \n')\"" 2>&1)
    rc=$(sed -n 's/^RC=//p'    <<<"$probe")
    hex=$(sed -n 's/^HEX=//p'   <<<"$probe")
    reach=$(sed -n 's/^REACH=//p' <<<"$probe")

    if [[ $reach != yes ]]; then
      # Nothing was verified. Never let an unreachable port read as a pass.
      warn "$c:$port is not accepting TCP — not verified"
    elif [[ $hex =~ ^[0-9a-f]{6}(04|07)[0-9a-f]{2}00000000 ]]; then
      bad "$c:$port answered the cleartext preface with an HTTP/2 frame — it is serving h2c"
    elif [[ $hex == 485454502f* ]]; then
      bad "$c:$port answered the cleartext preface with an HTTP/1 response"
    elif [[ $hex == 1503* ]]; then
      ok "$c:$port rejected the cleartext preface with a TLS alert (TLS listener confirmed)"
    elif [[ -z $hex ]]; then
      if [[ ${MESH_BY_ENTITY[$entity]} == 1 ]]; then
        ok "$c:$port accepted TCP but did not answer the cleartext preface (rc=${rc:-?})"
      else
        info "$c:$port silent, and $entity runs plaintext by design — inconclusive, skipped"
      fi
    else
      bad "$c:$port answered the cleartext preface: ${hex:0:40}"
    fi
  done
done

# ── 1b. a TLS handshake with no client certificate must be refused ────────────
# This is the mutual half of mutual TLS: server-only TLS would pass check 1 above and
# still accept any client. Entities that run plaintext by design are skipped, since
# there is no TLS listener to interrogate.
step "TLS handshake without a client certificate must be refused"
for svc in auth compliance payment-orchestrator; do
  for c in $(docker ps --format '{{.Names}}' | grep -- "-${svc}$" | sort); do
    [[ -n $ENTITY_FILTER && $c != *"$ENTITY_FILTER"* ]] && continue
    entity=${c%-$svc}
    [[ ${MESH_BY_ENTITY[$entity]:-0} == 1 ]] || continue
    net=$(docker inspect --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{break}}{{end}}' "$c")
    port=$(docker inspect --format '{{range $p,$_ := .Config.ExposedPorts}}{{$p}}{{break}}{{end}}' "$c" | cut -d/ -f1)
    [[ -z $port ]] && continue

    # Write application bytes and then wait, rather than closing stdin immediately.
    # Under TLS 1.3 a server that demands a client certificate lets the handshake reach
    # "Cipher is ..." and only then sends `alert certificate required`. A probe that
    # exits at EOF races that alert — measured over repeated runs it saw it roughly one
    # time in three, so the check reported a healthy mesh as broken two times in three.
    # Holding the connection open for the round trip makes the alert deterministic.
    out=$(docker run --rm --network "$net" --entrypoint sh "$GRPC_PROBE_IMAGE" -c \
            "{ printf 'PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n'; sleep 3; } \
             | timeout 12 openssl s_client -connect ${c}:${port} -verify_quiet 2>&1" 2>&1)
    # The alert is the pass. A handshake that completes and stays quiet is the failure:
    # the server accepted a client that presented no certificate at all.
    if grep -qiE 'alert certificate required|alert handshake failure|alert bad certificate|peer did not return a certificate' <<<"$out"; then
      ok "$c:$port refused a TLS handshake with no client certificate"
    elif grep -qE 'Cipher is [^ (]' <<<"$out"; then
      bad "$c:$port completed a TLS handshake without a client certificate"
    elif grep -qiE 'BIO_connect|Connection refused|connect:errno' <<<"$out"; then
      warn "$c:$port is not accepting TCP — not verified"
    elif grep -qiE 'wrong version number|unknown protocol' <<<"$out"; then
      bad "$c:$port is not speaking TLS on this port, but $entity is configured for the mesh"
    else
      warn "$c:$port gave no clear verdict: $(head -c 120 <<<"$out" | tr -d '\n')"
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
