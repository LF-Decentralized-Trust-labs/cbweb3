#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Self-test for verify-guard-mutations.sh: proves the harness reports the three failure
# shapes it exists to distinguish, instead of reporting every red suite as a detection.
#
# Each of these came from a false "the guard did not detect" in the guard-parity audit,
# and each cost real time. A harness that cannot tell them apart certifies nothing,
# including itself.
#
# Run with bash, never zsh: zsh does not fork the last stage of a pipeline, which is the
# class of bug that once let the licence gate pass without verifying anything.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 1

HARNESS=tools/verify-guard-mutations.sh
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
BEFORE="$(git status --porcelain)"
rc=0

expect() { # expect <label> <needle> <script-body>
  local label="$1" needle="$2" body="$3"
  printf '%s' "$body" > "$TMP/case.sh"
  local out
  out=$(bash "$TMP/case.sh" 2>&1)
  if grep -qF "$needle" <<<"$out"; then
    echo "ok    | $label"
  else
    echo "FALHA | $label — esperava $needle, saiu:"
    sed 's/^/        /' <<<"$out"
    rc=1
  fi
}

# The harness's helpers are sourced by running it with a sentinel that stops before the
# cases; simpler and less brittle is to exercise the same three shapes directly, using the
# harness's own functions copied by `sed` out of the file. Extracting rather than
# duplicating them is deliberate: a self-test written against a private copy of the logic
# would keep passing after the harness changed.
sed -n '/^apply()/,/^}/p;/^run_case()/,/^}/p' "$HARNESS" > "$TMP/lib.sh"
if [ ! -s "$TMP/lib.sh" ]; then
  echo "FALHA | não consegui extrair apply()/run_case() de $HARNESS — o self-test não testaria nada"
  exit 1
fi

PRELUDE='set -u
cd "$(git rev-parse --show-toplevel)" || exit 1
pass=0; fail=0; declare -a REPORT
A=scenario-a/toolkit
source '"$TMP"'/lib.sh
'
EPILOGUE='
printf "%s\n" "${REPORT[@]}"'

# 1. A mutation whose pattern does not match must abort the case, never pass silently.
expect "padrão que não casa vira HARNESS, não aprovação" "HARNESS" \
"$PRELUDE"'run_case "caso de teste" "$A" "TestProxyUpstreamsFitDNSLabel" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  "ESTA-STRING-NAO-EXISTE-EM-LUGAR-NENHUM" "substituta"'"$EPILOGUE"

# 2. A mutation that does not compile is INCONCLUSIVE. A red suite is not a detection when
#    the package never built: deleting a guard's subject once took the suite down by nil
#    panic and was misread as the guard working.
expect "mutação que não compila vira INCONCL." "INCONCL." \
"$PRELUDE"'run_case "caso de teste" "$A" "TestProxyUpstreamsFitDNSLabel" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  "func centralBankProxyRoutes(prefix string) []ProxyRoute {" \
  "func centralBankProxyRoutes(prefix string) []ProxyRoute { isto nao e Go valido"'"$EPILOGUE"

# 3. A -run that matches no test exits 0. Reported as a pass, it would certify a guard
#    that never ran.
expect "-run que não casa nenhum teste vira HARNESS" "HARNESS" \
"$PRELUDE"'run_case "caso de teste" "$A" "TestQueNaoExiste" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  "{Segment: \"bank\", Upstream: prefix + \"-bank-frontend:80\"}," \
  "{Segment: \"bank\", Upstream: prefix + \"-bank-spa:80\"},"'"$EPILOGUE"

# 4. And the positive control: a real mutation of a real guard must read DETECTOU, or the
#    three refusals above could be coming from a harness that refuses everything.
expect "mutação real de guarda real vira DETECTOU" "DETECTOU" \
"$PRELUDE"'run_case "caso de teste" "$A" "TestProxyUpstreamsNameContainersTheTemplatesCreate" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  "{Segment: \"bank\", Upstream: prefix + \"-bank-frontend:80\"}," \
  "{Segment: \"bank\", Upstream: prefix + \"-bank-spa:80\"},"'"$EPILOGUE"

if [ "$(git status --porcelain)" != "$BEFORE" ]; then
  echo "FALHA | o self-test deixou resíduo na árvore"
  rc=1
fi
[ "$rc" -eq 0 ] && echo "self-test do harness: ok"
exit "$rc"
