#!/usr/bin/env bash
# Generate ONE scenario fragment for an entity's launcher (config.<scenario>.json),
# listing only that entity's local portals in that scenario. Each scenario's toolkit
# writes its own fragment; the launcher merges them client-side. This is the standalone
# helper — the toolkits generate the same JSON internally during `apply`.
#
# The launcher is deployed per entity (on its own VPS), so this never references any
# other institution — adding a new bank/spoke means running this on the new VPS.
#
# Portal host ports are derived from the entity's Besu RPC port in the scenario by the
# toolkit's fixed frontend offsets:
#   Scenario A: governance/bank +17000, treasury +18000, supervisor +22000, noc +24000
#   Scenario B: governance/bank  +9000, treasury +13000, supervisor +14000, noc +12000
# Role portals per entity role:
#   commercial-bank : bank
#   central-bank    : A = governance,treasury,supervisor,noc ; B = governance,treasury,supervisor
#   hub             : B = governance,noc   (hub has no Scenario A)
#
# Usage:
#   gen-config.sh --scenario a --entity "Banco Itaú" --role commercial-bank \
#     --host localhost --rpc 8646 > configs/config.a.json
set -euo pipefail

scenario=""; entity=""; role=""; host="localhost"; rpc=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --scenario) scenario="$2"; shift 2;;
    --entity) entity="$2"; shift 2;;
    --role) role="$2"; shift 2;;
    --host) host="$2"; shift 2;;
    --rpc) rpc="$2"; shift 2;;
    *) echo "unknown arg: $1" >&2; exit 2;;
  esac
done

if [[ -z "$scenario" || -z "$entity" || -z "$role" || -z "$rpc" ]]; then
  echo "usage: $0 --scenario a|b --entity NAME --role commercial-bank|central-bank|hub --host HOST --rpc PORT" >&2
  exit 2
fi

declare -A OFF_A=( [governance]=17000 [bank]=17000 [treasury]=18000 [supervisor]=22000 [noc]=24000 )
declare -A OFF_B=( [governance]=9000  [bank]=9000  [treasury]=13000 [supervisor]=14000 [noc]=12000 )
declare -A LABEL=( [governance]="Governance" [treasury]="Treasury" [supervisor]="Supervisor" [noc]="NOC" [bank]="Bank Portal" )

case "$scenario" in
  a) S="A"; declare -n OFF=OFF_A;;
  b) S="B"; declare -n OFF=OFF_B;;
  *) echo "invalid scenario: $scenario (want a|b)" >&2; exit 2;;
esac

case "$role" in
  commercial-bank) roles="bank";;
  central-bank) [[ "$scenario" == a ]] && roles="governance treasury supervisor noc" || roles="governance treasury supervisor";;
  hub) [[ "$scenario" == b ]] && roles="governance noc" || { echo "hub has no Scenario A" >&2; exit 2; };;
  *) echo "invalid role: $role (want commercial-bank|central-bank|hub)" >&2; exit 2;;
esac

entries=()
for r in $roles; do
  port=$(( rpc + OFF[$r] ))
  entries+=("{ \"scenario\": \"$S\", \"role\": \"$r\", \"label\": \"${LABEL[$r]}\", \"url\": \"http://$host:$port\" }")
done

echo "{"
echo "  \"entity\": \"$entity\","
echo "  \"portals\": ["
n=${#entries[@]}
for i in "${!entries[@]}"; do
  sep=","
  [[ $i -eq $((n - 1)) ]] && sep=""
  echo "    ${entries[$i]}$sep"
done
echo "  ]"
echo "}"
