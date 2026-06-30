# Scenario A Toolkit — E2E Status

> Status of the `cbweb3` provisioning toolkit (`mode:found` / `mode:join`) as
> exercised end-to-end against real Docker. Updated as the E2E is built out.

## Summary

The toolkit provisions a CBDC spoke end-to-end:

- **`found`** — a central bank founds a spoke (Besu+Paladin, spoke contracts,
  node registry, relay registration, and its operational backend stack).
- **`join`** — a commercial bank joins dynamically and brings up its own full
  operational stack.

Both run idempotently (re-running `apply` resumes from the first incomplete step).

## What is validated (real Docker)

### `found` (central bank) — ✅ complete
`start-besu → deploy-contracts → gen-tls → render-configs → register-nodes →
start-paladin → create-zeto-token → onboard-registry → register-relay →
render-cb-env → start-cb-infra → provision-keycloak → start-cb-backend`

- Besu (QBFT) + Paladin up; spoke contracts + IdentityRegistry deployed; relay
  registered; join bundle emitted.
- CB operational stack: dedicated Postgres + Redis + Keycloak (central-bank +
  `cbweb3`/NOC realms) + the 4 backend services.
- **api-gateway `/healthz` → 200.** No private keys in the rendered env
  (`CB_PRIVATE_KEY` empty; signing via the KeyProvider abstraction).

### `join` (commercial bank) — ✅ provisioning complete
`write-genesis → start-besu-join → wait-sync → vote-qbft → gen-tls-join →
render-config-join → start-paladin-join → register-paladin-node →
render-bank-env → start-bank-infra → provision-bank-keycloak → start-backend`
then the **deferred soft tail**: `create-pente-context → deploy-fxa-pente →
proof-of-possession → gen-csr → request-cert → receive-cert`.

- Besu joins the shared spoke network, syncs, and is voted in as a QBFT validator.
- Paladin node brought up (dedicated ports) and registered on-chain.
- Bank operational stack: dedicated Postgres + Redis + Keycloak (bank realm) +
  the 4 backend services, in commercial-bank mode.
- **Bank api-gateway `/healthz` → 200.** `apply` reports `status: success`.

### Local cross-stack networking (feature 018)
For `environment: local`, bundle endpoints (built from the CB's in-Docker
advertised host) are adapted on the join side:
- Besu peers over the shared spoke network using the container-internal P2P port
  (validated: `peers ≥ 1`, sync, QBFT vote).
- The host-run toolkit reaches the CB JSON-RPC / api-gateway via published host
  ports (`localhost`); the bank backend reaches the CB via `host.docker.internal`.

## Deferred (soft, non-fatal — logged with an actionable message)

These run in the join's deferred tail; a failure does not block provisioning
because the bank is already fully operational and none are consumed by the backend.

### 1. Governance-gated identity
- `proof-of-possession` — participant registration in `IdentityRegistry` is
  `onlyRole(GOVERNANCE_ROLE)`; performed by the **CB on KYC approval**
  (`approve-kyc → setParticipant`), not by the bank.
- `request-cert` / `receive-cert` — the CB-signed PKI cert is a **runtime**
  identity credential issued only after a **governance KYC approval** (Governance
  Portal). `request-cert` succeeds (credential request accepted, Keycloak user +
  wallet created); `receive-cert` reports *awaiting governance KYC approval*.
  Re-running `apply` after approval resumes the flow.

### 2. Bilateral Pente / FXAgreement (US3) — blocked by a Paladin behaviour
`create-pente-context` / `deploy-fxa-pente` create the bilateral CB↔bank Pente
privacy group and deploy `FXAgreement` inside it. **Blocked** — see below.

## Known issue: cross-node `pgroup_createGroup` (Paladin v0.15.0-rc.1)

**Symptom.** `pgroup_createGroup` on the bank's Paladin, with members
`[funded_operator@<spoke>-cb, funded_operator@<spoke>-bank]`, hangs and never
resolves the remote CB node.

**Ruled out (with evidence):**
| Candidate | Finding |
|-----------|---------|
| Inter-Paladin network | both Paladins on the shared `cbweb3-<spoke>-besu` network, correct aliases |
| TLS / cert SAN | match the advertised hostnames |
| Paladin host ports | fixed (separate +1000 bands; no collision with the CB) |
| On-chain node registration | `reg_queryEntries` returns both nodes |
| CB transport endpoint | registered: `dns:///paladin-<spoke>-cb:9000` + issuer cert |
| Client 30s timeout | removed (full 5-min context) — still fails |
| Timing / Paladin restart | no change |
| Block indexer lag | bank Besu, CB Besu and the Paladin indexer all at chain head |
| Registry DB row | the bank's own `paladin.db` resolves the CB node |

**The anomaly.** With everything above in order, the bank Paladin's
`registrymanager` runs exactly:
```sql
SELECT * FROM reg_entries WHERE registry='evm-registry'
  AND name='<spoke>-cb' AND parent_id IS NULL AND active IS TRUE LIMIT 1   -- rows:0
```
and gets **0 rows**, while the **same query on the bank's own `paladin.db`
(including WAL) returns 1** (the row exists: `parent_id` NULL, `active=1`). The
operation never proceeds to the transport step.

**Hypothesis.** A divergence between the `registrymanager`'s in-memory resolution
view (derived from `IdentityRegistered` events, where `parentHash = zeroHash`) and
the persisted state (`parent_id` NULL). Consistent with the restart not helping
(the view is rebuilt from the same events). This path is not exercised by the
spk-02 spike (which validated cross-node transport only via
`reg_queryEntriesWithProps`, a read) nor by the Scenario B reference (which creates
Pente groups among nodes local to one stack).

**Next steps (outside black-box trial-and-error):**
1. Confirm cross-node member-resolution semantics with the Paladin source /
   maintainers (open an issue with the evidence above).
2. Test registering nodes with `parentIdentityHash` ≠ `zeroHash`.

The bank's backend boots without the bilateral `FXAgreement`, so this does not
gate provisioning; the toolkit reports `success` with these steps `pending`.
