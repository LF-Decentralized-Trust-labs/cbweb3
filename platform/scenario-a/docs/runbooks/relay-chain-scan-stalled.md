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

These causes produce the same symptom and need different repairs.

```bash
docker logs --since 30m "$CACTI" 2>&1 | grep -E \
  'chain head .* is behind|poll cycle error|SettleHTLC gRPC failed'
```

| What you see | Cause | Go to |
| --- | --- | --- |
| `chain head N is behind resume block M` | the watermark is ahead of the chain, typically after a reset | [B](#b-watermark-ahead-of-the-chain) |
| `poll cycle error`, repeating | the relay cannot reach that spoke's Besu | [C](#c-the-spokes-besu-is-unreachable) |
| `SettleHTLC gRPC failed … NOT_FOUND` | **you are on an old relay.** See [A](#a-historical-a-settlement-that-can-never-succeed) |
| `events lost to journal retention` (in an ORCHESTRATOR log, not the relay's) | this entity fell behind the capped journal and missed settlements | [D](#d-a-consumer-fell-behind-the-journal) |

---

### A (historical). A settlement that can never succeed

**This stall cannot happen on a current relay.** The settle push was removed: the relay
appends the claim, secret included, to the journal, and the leg's owner settles itself by
polling it. Nothing in the settle path makes a network call, so nothing can fail, so
nothing can hold the watermark. If you are seeing `SettleHTLC gRPC failed`, the relay
container is running an older image — check its version before repairing anything.

The section is kept because a long-lived deployment can still be on that image, and
because the store it leaves behind is repaired the same way.

**Why it stalled.** A failed settlement held the block watermark at its own block so the
claim was retried instead of skipped. The hold was on the BLOCK, so it also froze every
unrelated event after it — locks included — on that whole spoke.

The failure was not transient. The relay keeps one gRPC endpoint per spoke and it is that
spoke's **central bank** orchestrator, while an inter-bank leg lives on a **commercial
bank's**. `SettleHTLC` answered `NOT_FOUND` and always would — and no endpoint would have
worked, because settling a leg is `transferLocked` on the owner's own Paladin node.

A count tells you how long it had been going:

```bash
docker logs "$CACTI" 2>&1 | grep -c 'SettleHTLC gRPC failed'
```

On LNET this read **167,360**.

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

**Get the key right, or the repair is a no-op.** The store holds two HTLC key shapes and
only one of them releases the watermark:

| Key | What it does |
| --- | --- |
| `htlc-evt:<observing-spoke>:<txHash>:<logIndex>` | **this one.** The per-event guard the poll loop tests before processing a claim. |
| `htlc-settled:<dest-spoke>:<dest-contract>` | echo guard only. Consulted when the *destination* spoke is scanned, to skip the settlement bouncing back. Writing it does nothing for a held watermark. |

Take the values from the **`LogHTLCClaimed`** line, not from the `SettleHTLC gRPC failed`
line — the failure line prints the counterpart's contract id and carries no txHash:

```
[spoke-costa-rica] LogHTLCClaimed contractId=34ce0a59… block=76018 tx=0x9f2c…
 ^ observing spoke                                                 ^ txHash
```

`logIndex` is not in that line. It is `0` unless the same transaction emitted more than one
`LogHTLCClaimed`; confirm against the keys the store already holds (below) or read the
receipt with `eth_getTransactionReceipt`.

```bash
# The keys already in the store — copy the shape from a claim that DID deliver.
docker run --rm -v "$VOL":/data alpine:3.23 cat /data/cacti-relay-store.json \
  | python3 -c 'import sys,json; print([k for k in json.load(sys.stdin)["delivered"] if k.startswith("htlc-evt:")][:10])'
```

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
  "htlc-evt:<observing-spoke>:<txHash>:<logIndex>",
  # one line per stuck claim, from the LogHTLCClaimed lines above
]:
    d.setdefault("delivered", {})[k] = now
    # Forget its failure history too, so a later forced retry is not born at the cap.
    d.get("settleFailures", {}).pop(k, None)
json.dump(d, open(p, "w"))
PY'

docker start "$CACTI"
```

**Confirm it worked** — the watermark must move within a poll interval or two:

```bash
docker run --rm -v "$VOL":/data alpine:3.23 cat /data/cacti-relay-store.json \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["watermarks"])'
```

If it has not moved, the key did not match. Do not repeat the edit with more keys: list the
store's keys as above and compare character by character against the spoke id in the
`LogHTLCClaimed` line.

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

### D. A consumer fell behind the journal

This one appears in a **payment-orchestrator** log, not the relay's:

```
ERROR cacti: events lost to journal retention — this entity was too far behind the relay
and the events it missed no longer exist  kind=settle cursor=… lost_from_seq=…
lost_through_seq=…
```

The journal is capped at 10,000 entries per kind. An entity down long enough for that many
events to pass loses the ones that fell off, and they cannot be recovered from the relay —
they are gone. For `kind=settle` each lost event is a counterpart leg that will not settle
itself, because the journal is how its owner finds out it must.

It is not a stall: the orchestrator reports the range once and carries on with what
survived. The repair is reconciliation, not a restart:

```bash
# Legs still LOCKED whose counterpart has been claimed on the other spoke.
curl -sk -b "access_token=$TOK" "https://<entity-host>/a/api/v1/htlc/search" \
  | python3 -c 'import sys,json; [print(h["contract_id"], h["state"], h["hash_lock"]) for h in json.load(sys.stdin).get("htlcs",[]) if h["state"]=="HTLC_STATE_LOCKED"]'
```

For each, find the secret in the counterpart spoke's `LogHTLCClaimed` on-chain and settle
the leg directly. If the timelock has passed, refund instead — see below.

If this appears routinely rather than after an outage, the cap is too small for the
traffic, or an entity is restarting more often than it should.

## After the removal of the push

A current relay logs neither `SettleHTLC gRPC failed` nor `giving up on this event`; both
belonged to the push. What it logs on a claim is the journal append:

```
[spoke-x] LogHTLCClaimed contractId=… block=… tx=…
```

and the settlement then happens in the destination entity, visible in ITS log:

```
relay settle: local HTLC leg settled  contractId=<the OTHER spoke's contract id>
```

The contract id there is the counterpart's, not its own — the entity resolves its own leg
by `sha256(secret)`. That is the line that proves the path works end to end.

## Recovering a trade that expired meanwhile

Nothing is lost. Each leg has its own timelock and is refundable after it. The trade has
to be proposed again — an expired HTLC cannot be revived, and re-locking against the old
agreement will be refused.

## Related

- [pente-endorser-key-loss-recovery.md](pente-endorser-key-loss-recovery.md) — a
  different silent failure with a similarly unbounded gap between cause and symptom.
- [container-log-disk-recovery.md](container-log-disk-recovery.md) — a relay looping on
  a permanent failure also writes a great deal of log; check disk after a long stall.
