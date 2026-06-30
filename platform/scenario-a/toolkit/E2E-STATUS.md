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

### 2. Bilateral Pente / FXAgreement (US3)
`create-pente-context` / `deploy-fxa-pente` create the bilateral CB↔bank Pente
privacy group and deploy `FXAgreement` inside it. The cross-node transport that
these need is now **working** (see resolved issue below). `create-pente-context`
opens with a **peer-readiness gate** (`waitPentePeersReady`): it probes each member
with `ptx_resolveVerifier` until the remote node's mTLS transport is up before
creating the group, so the step no longer races the join's soft tail. The gate
distinguishes transient transport errors (peer still connecting → retry until the
step timeout) from permanent ones (e.g. a cert node-name mismatch → fail fast).
Validated on the live two-spoke env: all four banks created their Pente group +
deployed FXAgreement after the gate reported `Paladin peer … ready`.

## Resolved: cross-node Paladin transport — cert CN ≠ node name (fixed 2026-06-30)

**Symptom (before fix).** `pgroup_createGroup` on the bank's Paladin with members
`[funded_operator@<spoke>-cb, funded_operator@<spoke>-bank]` hung forever; a group
with only the local member succeeded instantly. Any cross-node transport op
(`ptx_resolveVerifier` to a remote identity) failed.

**Root cause: the Paladin gRPC transport cert's TLS identity did not match the
registered node name.** The transport authenticates a peer by matching the cert
identity against the EXPECTED NODE NAME from the registry. The toolkit minted the
cert with `CN = paladin-<spoke>-<node>` (the container hostname) while the node is
registered as `<spoke>-<node>`, so the handshake was rejected:
```
PD030011: the TLS identity of the node 'paladin-spoke-brl-cb'
          does not match the expected node 'spoke-brl-cb'
```
(The `tls: bad record MAC` / `EOF` lines previously seen in the CB log were noise
from a plain `curl` probe with no matching client cert — `ptx_resolveVerifier`
surfaced the real `PD030011`. Registry resolution, TCP reachability, and cert
material were all already correct — the gap was purely the cert subject/SAN.)
This was inherited from the spk-02 spike, whose T3 never ran a real cross-node
group op and whose log grep did not match `PD030011`, so the gap went undetected.

**Fix.** `gen-tls` (CB) and `gen-tls-join` (bank) now set `CN = <node name>`
(`cbNodeName` / `bankNodeName`) and keep the container hostname in the SAN
(`[<node name>, paladin-<spoke>-<node>, localhost]`) so the `dns:///` dial still
validates. See `step_gen_tls.go` / `step_gen_tls_join.go`.

**Validated (clean rebuild from scratch — images/volumes/data wiped, 2026-06-30):**
- transport certs now carry `CN = spoke-brl-cb` / `CN = spoke-brl-bank-itau`;
- `ptx_resolveVerifier` cross-node (CB↔itau) returns the eth address (was PD030011);
- `pgroup_createGroup` `[funded_operator@spoke-brl-cb, funded_operator@spoke-brl-bank-itau]`
  creates the group (was a hang);
- no `handshake failed` / `bad record MAC` / `PD030011` in the Paladin logs.
