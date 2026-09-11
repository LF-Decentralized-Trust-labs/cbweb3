<!-- SPDX-License-Identifier: Apache-2.0 -->

# Runbook — the relay stopped scanning, and PvP went quiet

**Applies to:** any Scenario A deployment whose Cacti HTLC relay has stopped advancing
its block watermark. Most likely on a multi-VM stack where commercial banks settle
between spokes.

**Why it exists:** this failure does not look like a relay failure. It looks like
"PvP stopped working". Both legs of a trade lock on-chain, both portals show the trade
pending, `counterparty_locked` is `false` on both sides, and no service logs an error.
The relay itself looks healthy — it is logging busily, just about the same event. On
LNET it cost three days and a trade left to expire before anyone looked at the
watermark.

## Symptom

From an operator: a PvP settlement stays pending on both sides. The second bank
"continued" with the settlement code and nothing happened.

The signature, on either bank's gateway:

```bash
curl -sk -b "access_token=$TOK" \
  "https://<bank-host>/a/api/v1/htlc/search" | python3 -m json.tool
```

Both banks hold a leg for the same `hash_lock`, both with
`"counterparty_locked": false`. That combination is the tell: each side locked, and
neither learned about the other. Only the relay carries that news.

## Confirm it in one check

The watermark is the relay's block cursor. If it does not move while the chain
produces blocks, the relay is not scanning.

```bash
# on the relay host
CACTI=$(docker ps --format '{{.Names}}' | grep -i cacti | grep -v liquidity | head -1)
VOL=$(docker inspect "$CACTI" \
  --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}')

for i in 1 2; do
  docker run --rm -v "$VOL":/data alpine:3.23 cat /data/cacti-relay-store.json \
    | python3 -c 'import sys,json; print(json.load(sys.stdin)["watermarks"])'
  sleep 30
done
```

Two identical readings on a live chain means stalled.

## Which stall is it

Three causes produce the same symptom and need different repairs.

```bash
docker logs --since 30m "$CACTI" 2>&1 | grep -E \
  'giving up on this event|chain head .* is behind|poll cycle error|SettleHTLC gRPC failed'
```

| What you see | Cause | Go to |
| --- | --- | --- |
| `SettleHTLC gRPC failed … NOT_FOUND`, repeating without end | a settlement that can never succeed is holding the watermark | [A](#a-a-settlement-that-can-never-succeed) |
| `chain head N is behind resume block M` | the watermark is ahead of the chain, typically after a reset | [B](#b-watermark-ahead-of-the-chain) |
| `poll cycle error`, repeating | the relay cannot reach that spoke's Besu | [C](#c-the-spokes-besu-is-unreachable) |
| `giving up on this event` | the bound is working: the relay dropped one settlement and moved on | no stall — see [after the fix](#after-the-bounded-hold) |

A count is worth taking, because it tells you how long this has been going:

```bash
docker logs "$CACTI" 2>&1 | grep -c 'SettleHTLC gRPC failed'
```

On LNET this read **167,360**.

---

### A. A settlement that can never succeed

**Why it stalls.** A failed settlement holds the block watermark at its own block so the
claim is retried instead of skipped. The hold is on the BLOCK, so it also freezes every
unrelated event after it — locks included — on that whole spoke.

The failure is usually not transient. The relay keeps one gRPC endpoint per spoke and it
is that spoke's **central bank** orchestrator, while an inter-bank leg lives on a
**commercial bank's**. `SettleHTLC` then answers `NOT_FOUND` and always will.

**Before repairing, check whether the value already moved.** A claim event means the
secret was revealed on-chain. Both legs may already be settled while the relay is still
retrying its own redundant push:

```bash
curl -sk -b "access_token=$TOK" \
  "https://<bank-host>/a/api/v1/htlc/status/<contract-id>" | python3 -m json.tool
```

`HTLC_STATE_SETTLED` on both sides means the swap completed and only the relay is stuck.

**Repair.** Mark the stuck claims as delivered so the watermark is released. This
records a fact you have verified — it does not skip a settlement that still has to
happen.

```bash
docker stop "$CACTI"

docker run --rm -v "$VOL":/data alpine:3.23 \
  sh -c 'cp /data/cacti-relay-store.json /data/cacti-relay-store.json.bak-$(date +%s)'

docker run --rm -v "$VOL":/data alpine:3.23 sh -c 'apk add --no-cache python3 >/dev/null && python3 - <<PY
import json, time
p = "/data/cacti-relay-store.json"
d = json.load(open(p))
now = int(time.time() * 1000)
for k in [
  "htlc-settled:<dest-spoke-id>:<dest-contract-id>",
  # one line per stuck claim, from the SettleHTLC failures above
]:
    d.setdefault("delivered", {})[k] = now
json.dump(d, open(p, "w"))
PY'

docker start "$CACTI"
```

If the loop persists, list the keys the store actually holds and match the shape:

```bash
docker run --rm -v "$VOL":/data alpine:3.23 cat /data/cacti-relay-store.json \
  | python3 -c 'import sys,json; print(list(json.load(sys.stdin)["delivered"])[:10])'
```

---

### B. Watermark ahead of the chain

Normal after a chain reset that the genesis guard did not catch: the volume kept a
watermark from the old chain. Lower it below the current head.

```bash
docker stop "$CACTI"
docker run --rm -v "$VOL":/data alpine:3.23 sh -c 'apk add --no-cache python3 >/dev/null && python3 - <<PY
import json
p = "/data/cacti-relay-store.json"
d = json.load(open(p))
d["watermarks"]["<spoke-id>"] = 0
json.dump(d, open(p, "w"))
PY'
docker start "$CACTI"
```

Setting it to `0` re-scans from genesis. The per-event `delivered` guards make that safe
— nothing already forwarded is forwarded twice — but it is slow on a long chain; prefer
a block shortly before the incident when you know one.

---

### C. The spoke's Besu is unreachable

`poll cycle error` repeating means the connector cannot reach that spoke. The relay
rebuilds the connector after three consecutive failures on its own, so a stall here is
usually the spoke, not the relay: check the Besu container on that VM, then the RPC URL
the relay has registered for it.

```bash
docker run --rm -v "$VOL":/data alpine:3.23 cat /data/cacti-spoke-registry.json \
  | python3 -m json.tool
```

---

## After the bounded hold

From the fix in `fix/scenario-a-relay-watermark-head-of-line`, a claim that fails
`MAX_SETTLE_ATTEMPTS` times is given up on, at error level, and the scan advances:

```
[spoke-x] settlement for contractId=… on spoke-y failed 20 times — giving up on this
event so the chain scan can advance. The counterpart leg was NOT settled by the relay
and needs an operator: check that spoke-y's registered gRPC endpoint serves the entity
that holds contractId=…
```

That line is **not** a stall. It means one settlement push was dropped and everything
else kept working. Note that the push is redundant with the journal: the claim, with its
secret, is appended to the settle journal BEFORE the push is attempted, and the
destination orchestrator pulls it and settles its own leg. In practice the leg settles
anyway — verify with `/htlc/status` rather than assuming either way.

What the line does tell you is that the endpoint-per-spoke routing is still in place. It
should stop appearing once that is addressed.

## Recovering a trade that expired meanwhile

Nothing is lost. Each leg has its own timelock and is refundable after it. The trade has
to be proposed again — an expired HTLC cannot be revived, and re-locking against the old
agreement will be refused.

## Related

- [pente-endorser-key-loss-recovery.md](pente-endorser-key-loss-recovery.md) — a
  different silent failure with a similarly unbounded gap between cause and symptom.
- [container-log-disk-recovery.md](container-log-disk-recovery.md) — a relay looping on
  a permanent failure also writes a great deal of log; check disk after a long stall.
