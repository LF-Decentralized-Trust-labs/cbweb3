# Research: SP02 — Live Spoke Join (QBFT + Paladin)

**Branch**: `019-spk02-live-join` | **Date**: 2026-06-26
**Status**: Pending POC execution — decisions below are informed hypotheses; each must be confirmed by running the spike

---

## U1 — QBFT Epoch Boundary and Vote Activation Timing

### What we need to know

In Besu 25.8.0 with QBFT block-header voting: default epoch length, and the exact block at
which a voted-in validator becomes active.

### Known from Besu 25.8.0 docs and source

- Default QBFT epoch length: **30,000 blocks** (configurable via `genesis.json` `epochlength` field)
- For the spike, `epochlength` must be set to a small value (e.g., 10 or 30 blocks) so the
  activation can be observed within minutes; the default 30,000 would take hours at 2s/block
- Vote threshold: **floor(N/2) + 1** of the current active validator set must vote in favor
  — for N=3 existing validators, **2 votes** suffice (not all 3)
- Vote activation: the new validator becomes active at the **next epoch boundary** after the
  threshold is crossed; if threshold is crossed mid-epoch, activation waits until the current
  epoch ends
- The RPC `qbft_getValidatorsByBlockNumber("pending")` shows the candidate before the epoch
  boundary; `qbft_getValidatorsByBlockNumber("latest")` reflects post-activation state

### POC validation task

Set `epochlength: 30` in genesis. Cast 2 votes. Poll `qbft_getValidatorsByBlockNumber` every
block. Record the block number where the new address first appears — confirm it aligns with an
epoch boundary (`blockNumber % 30 == 0`).

### Decision (pending POC confirmation)

```
Epoch length (spike):     30 blocks (must be set in genesis; default 30000 is impractical)
Vote threshold (N=3):     2 votes (floor(3/2)+1 = 2)
Activation timing:        next epoch boundary after threshold block
Verification RPC:         qbft_getValidatorsByBlockNumber("latest")
```

---

## U2 — Paladin TLS Trust Model

### What we need to know

When Paladin node A receives an mTLS connection from a new node B whose self-signed cert
was not present at A's startup: does the handshake succeed or fail?

### Analysis from Paladin config schema and architecture

The `generate-certs.sh` in the project creates **individual self-signed certs per node** —
there is no shared CA. The Paladin gRPC transport config has:

```yaml
transports:
  grpc:
    config:
      tls:
        certFile: /etc/paladin/tls.crt
        keyFile:  /etc/paladin/tls.key
```

No `caFile` or `trustAllCerts` field is visible in the project's config templates. This
leaves three possibilities:

| Hypothesis | Mechanism | Expected behavior |
|------------|-----------|------------------|
| **H-A (Registry-fetched cert)** | Paladin queries the on-chain registry for the peer's cert before the mTLS handshake | New node registers cert on-chain → existing nodes trust it automatically; **no restart** |
| **H-B (Startup-only trust store)** | Paladin loads peer certs from a config file at startup; file is not hot-reloaded | New cert not in existing nodes' trust store → `x509: certificate signed by unknown authority`; **restart required** after adding cert to config |
| **H-C (InsecureSkipVerify)** | Paladin uses `insecureSkipVerify: true` for inter-node transport (common in private networks) | All self-signed certs accepted regardless; **no restart** |

### Evidence collection tasks for the POC

1. Start CB + Bank-A Paladin with their existing certs
2. Start Bank-X Paladin with a newly generated self-signed cert (not pre-shared with existing nodes)
3. Register Bank-X on-chain
4. Attempt `ptx_getTransaction` forwarded from CB to Bank-X — observe response:
   - `200 OK` → H-A or H-C (no restart needed)
   - `transport: x509` error in logs → H-B (restart needed)
5. If H-B: check if Paladin config supports a `caFile` or `trustedCerts` list that can be
   updated without restart (hot-reload). If not: document rolling restart procedure.

### Decision (pending POC execution)

```
TLS outcome:   [PENDING — to be filled after step 4 above]
Evidence:      [PENDING — log excerpt or success response to be recorded here]
Restart scope: [PENDING — if H-B: Paladin CB + Bank-A only; Besu nodes unaffected]
```

---

## U3 — Paladin On-Chain Registry: Reactive vs. Startup-Only Discovery

### What we need to know

After existing Paladin nodes are running, do they watch for new `PaladinRegisterTransport`
events on the registry contract and immediately update their peer list, or do they only query
the registry at startup?

### Analysis from Paladin architecture

Paladin uses a `registries.evm-registry` plugin (`libevm.so`) that points to a contract
address. The plugin pattern in Paladin Core is to subscribe to contract events using the
Besu block listener — this is the same mechanism Paladin uses for private transaction indexing.
If the registry plugin follows the same event-subscription pattern, **discovery is reactive**.

However, peer discovery and peer trust are separate concerns:
- Discovery (knowing the new node exists and its endpoint): likely reactive if event-driven
- Trust (accepting the new node's TLS cert): separate, depends on U2

### Evidence collection tasks for the POC

1. After Bank-X registers on-chain, check CB Paladin logs for a line mentioning:
   - Bank-X `nodeName` (e.g., `spoke-a-bank-x`)
   - Bank-X gRPC endpoint (`paladin-spoke-a-bank-x:9000`)
   - Any "peer discovered" / "transport registered" message
2. Query Paladin's peer list API (if available) before and after Bank-X registration
3. Record the delay between on-chain registration block and first log appearance

### Decision (pending POC execution)

```
Discovery mode: [PENDING — reactive-event-driven / startup-only]
Evidence:       [PENDING — log excerpt to be recorded here]
Cooldown:       [PENDING — seconds from registration block to peer appearing in logs]
```

---

## Resolved: Spike dependencies on SP01

| Item | SP01 decision | SP02 reuse |
|------|--------------|-----------|
| Enode addressing | `--nat-method=DOCKER` for cross-compose | Reused exactly in `stack-found.yml` and `stack-join.yml` |
| Genesis idempotency | `genesis-once.sh` pattern | Reused; genesis generated once including `epochlength: 30` |
| `--bootnodes` resolution | `validator-entry.sh` resolves bootnode container IP via Docker DNS | Reused for validators 1–2; joiner uses bundle enode |
| Bundle format | `cbweb3/v1 JoinBundle` YAML schema | Reused; bundle is consumed by `join-besu.sh` |
| Cross-compose isolation | `extra_hosts: host.docker.internal:host-gateway` | Reused in `stack-join.yml` |
| Cacti relay model | Not in scope for SP02 | Excluded from SP02 |
