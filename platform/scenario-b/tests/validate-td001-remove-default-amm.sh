#!/usr/bin/env bash
# TD-001 validation on a clean deploy (no default AMM).
# 1) hub has no default AMM (registry empty + AMM_CONTRACT_ADDRESS unset in gateway)
# 2) propose(no amm)->dedicated AMM->confirm->assert W-tokens
# 3) mint&approve + seed 18-dec
# 4) swap itau->macro COMPLETED (resolver-only path, no default AMM)
# 5) breaker pause on the pair -> the pair's DEDICATED AMM isPaused()==true on-chain
set -uo pipefail
export PATH="$PATH:/home/antonio-goledger/.foundry/bin"
HUB=http://localhost:16845; BRCB=http://localhost:16645; ARCB=http://localhost:16745
ITAU=http://localhost:16646; MACRO=http://localhost:16747; HUBRPC=http://localhost:8845
PR=0xfeae27388a65ee984f452f86effed42aabd438fd
PASS=0; FAIL=0
ok(){ echo "  ✓ $*"; PASS=$((PASS+1)); }
bad(){ echo "  ✗ $*"; FAIL=$((FAIL+1)); }
jget(){ python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('$1','') if isinstance(d,dict) else '')" 2>/dev/null; }
login(){ curl -s -m 30 -X POST "$1/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"clientId\":\"$2\",\"clientSecret\":\"$3\"}" | jget accessToken; }

echo "== no default AMM =="
AMMENV=$(docker exec cbweb3-central-bank-brazil-central-bank-api-gateway printenv AMM_CONTRACT_ADDRESS 2>/dev/null)
[ -z "$AMMENV" ] && ok "AMM_CONTRACT_ADDRESS unset in CB gateway" || bad "AMM_CONTRACT_ADDRESS still set ($AMMENV)"
ALLP=$(cast call $PR 'getAllPairs()(bytes32[])' --rpc-url $HUBRPC 2>/dev/null)
echo "  getAllPairs (raw): $ALLP"

echo "== login =="
BRT=$(login $BRCB admin@brasil.governance.gov brasil-governance-local); [ -n "$BRT" ] && ok "BR CB login" || bad "BR CB login"
ART=$(login $ARCB admin@argentina.governance.gov argentina-governance-local); [ -n "$ART" ] && ok "AR CB login" || bad "AR CB login"

echo "== currencies =="
CUR=$(curl -s -b "access_token=$BRT" "$BRCB/api/v2/hub/currencies")
pick(){ python3 -c "
import sys,json
d=json.load(sys.stdin); arr=d.get('currencies',d) if isinstance(d,dict) else d
for c in arr:
    if '$1' in (c.get('symbol') or ''): print((c.get('token_address') or '')+'|'+(c.get('proposer_cb') or '')); break
"; }
BRL=$(printf '%s' "$CUR" | pick BRL); ARS=$(printf '%s' "$CUR" | pick ARS)
TOKA=${BRL%%|*}; CBA=${BRL##*|}; TOKB=${ARS%%|*}; CBB=${ARS##*|}
[ -n "$TOKA" ] && [ -n "$TOKB" ] && ok "W-tokens resolved (BRL=$TOKA ARS=$TOKB)" || bad "currencies missing"

PAIR=W-tCeBM_BRL-W-tCeBM_ARS
echo "== propose (NO amm_address) =="
PRr=$(curl -s -b "access_token=$BRT" -X POST "$BRCB/api/v2/amm/pairs/propose" -H 'Content-Type: application/json' \
  -d "{\"pair_id\":\"$PAIR\",\"token_a_address\":\"$TOKA\",\"token_b_address\":\"$TOKB\",\"proposer_cb\":\"$CBA\"}")
echo "  $PRr"
AMM=$(printf '%s' "$PRr" | jget amm_address)
[ -n "$AMM" ] && ok "dedicated AMM deployed on propose: $AMM" || bad "propose (no amm) failed"
curl -s -b "access_token=$ART" -X POST "$ARCB/api/v2/amm/pairs/confirm" -H 'Content-Type: application/json' -d "{\"pair_id\":\"$PAIR\",\"confirmer_cb\":\"$CBB\"}" -o /tmp/cf.out
[ "$(jget status < /tmp/cf.out)" = ACTIVE ] && ok "pair ACTIVE" || bad "confirm"
ta=$(cast call "$AMM" 'TOKEN_A()(address)' --rpc-url $HUBRPC 2>/dev/null|head -1)
[ "${ta,,}" = "${TOKA,,}" ] && ok "AMM bound to W-tokens" || bad "AMM token mismatch ($ta)"

echo "== mint&approve + seed (18-dec) =="
L=1000000000000000000000
for s in A B; do
  r=$(curl -s -o /dev/null -w "%{http_code}" -b "access_token=$BRT" -X POST "$BRCB/api/v2/amm/token/mint-and-approve" -H 'Content-Type: application/json' -d "{\"pool_pair\":\"$PAIR\",\"amount\":\"$L\",\"side\":\"$s\"}")
  [ "$r" = 200 ] && ok "mint&approve $s" || bad "mint&approve $s ($r)"
done
r=$(curl -s -o /dev/null -w "%{http_code}" -b "access_token=$BRT" -X POST "$BRCB/api/v2/amm/liquidity/add" -H 'Content-Type: application/json' -d "{\"pool_pair\":\"$PAIR\",\"provider_bank_id\":\"central-bank\",\"token_a_amount\":\"$L\",\"token_b_amount\":\"$L\"}")
[ "$r" = 201 ] && ok "seed 201" || bad "seed ($r)"

echo "== onboard itau + macro (ACTIVE) =="
onboard(){ curl -s -b "access_token=$2" -X POST "$1/api/v1/onboarding/initiate" -H 'Content-Type: application/json' -d "{\"institution_name\":\"$5\",\"country\":\"$6\",\"role\":\"ROLE_COMMERCIAL_BANK\",\"email\":\"$7\",\"username\":\"$8\"}" >/dev/null
  local ms rid uid; ms=$(curl -s -b "access_token=$2" "$1/api/v1/onboarding/my-status"); rid=$(printf '%s' "$ms"|jget request_id); uid=$(printf '%s' "$ms"|jget user_id)
  curl -s -b "access_token=$4" -X POST "$3/api/v1/compliance/approve-kyc" -H 'Content-Type: application/json' -d "{\"subject\":\"$uid\",\"reason\":\"v\"}" >/dev/null
  curl -s -b "access_token=$2" -X POST "$1/api/v1/onboarding/complete" -H 'Content-Type: application/json' -d "{\"request_id\":\"$rid\",\"user_id\":\"$uid\"}" >/dev/null
  sleep 2; curl -s -b "access_token=$2" "$1/api/v1/onboarding/my-status" | jget status; }
IT=$(login $ITAU admin@itau.brasil.com itau-bank-local); MT=$(login $MACRO admin@macro.argentina.com macro-bank-local)
[ "$(onboard $ITAU "$IT" $BRCB "$BRT" 'Banco Itau' BR o@itau.br u-itau)" = ACTIVE ] && ok "itau ACTIVE" || bad "itau onboarding"
[ "$(onboard $MACRO "$MT" $ARCB "$ART" 'Banco Macro' AR o@macro.ar u-macro)" = ACTIVE ] && ok "macro ACTIVE" || bad "macro onboarding"

echo "== deposit+tokenize itau (18-dec) =="
d=$(curl -s -b "access_token=$IT" -X POST "$ITAU/api/v1/payments/deposits" -H 'Content-Type: application/json' -d '{"amount":"10000000000000000000000"}' | jget deposit_id)
curl -s -b "access_token=$BRT" -X POST "$BRCB/api/v1/payments/deposits/approve" -H 'Content-Type: application/json' -d "{\"deposit_id\":\"$d\"}" >/dev/null; sleep 3
e=$(curl -s -b "access_token=$IT" -X POST "$ITAU/api/v1/payments/escrows" -H 'Content-Type: application/json' -d "{\"deposit_id\":\"$d\",\"amount\":\"5000000000000000000000\"}" | jget escrow_id)
curl -s -b "access_token=$BRT" -X POST "$BRCB/api/v1/payments/escrows/approve" -H 'Content-Type: application/json' -d "{\"escrow_id\":\"$e\"}" >/dev/null; sleep 3
tb=$(curl -s -b "access_token=$IT" "$ITAU/api/v1/token/balance" | jget balance); [ -n "$tb" ] && ok "itau tCeBM=$tb" || bad "tokenize"

echo "== swap itau->macro (resolver-only, no default AMM) =="
sw=$(curl -s -b "access_token=$IT" -X POST "$ITAU/api/v2/amm/swap/cross-currency" -H 'Content-Type: application/json' \
  -d "{\"source_currency\":\"BRL\",\"target_currency\":\"ARS\",\"pool_pair\":\"$PAIR\",\"amount_out\":\"5000000000000000000\",\"max_amount_in\":\"6000000000000000000\",\"beneficiary_bank_id\":\"bank-macro\"}")
echo "  $sw"; [ "$(printf '%s' "$sw" | jget status)" = COMPLETED ] && ok "SWAP COMPLETED" || bad "swap"

echo "== breaker pause targets the pair's DEDICATED AMM =="
pp=$(curl -s -o /dev/null -w "%{http_code}" -b "access_token=$BRT" -X POST "$BRCB/api/v2/governance/circuit-breaker/pause" -H 'Content-Type: application/json' -d "{\"pair\":\"$PAIR\",\"bank_id\":\"central-bank-brazil\",\"reason_code\":\"TD001-TEST\"}")
echo "  pause HTTP $pp"
sleep 2
paused=$(cast call "$AMM" 'isPaused()(bool)' --rpc-url $HUBRPC 2>/dev/null | head -1)
[ "$paused" = true ] && ok "dedicated AMM $AMM isPaused()==true (per-pair breaker)" || bad "breaker did not pause the dedicated AMM (isPaused=$paused)"

echo "== RESULT: PASS=$PASS FAIL=$FAIL =="
