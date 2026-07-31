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
start-paladin → create-zeto-token → onboard-registry → deploy-fiat-token →
deploy-htlc → register-relay → render-cb-env → start-cb-infra →
provision-keycloak → start-cb-backend → start-cb-frontend`

- Besu (QBFT) + Paladin up; spoke contracts + IdentityRegistry deployed; relay
  registered; join bundle emitted.
- Besu-layer settlement contracts deployed and wired (see "Besu-layer settlement
  path" below): `FiatCentralBankMoney` (fCeBM) + `HashTimeLockedContract` (HTLC).
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

### Besu-layer settlement path (fCeBM / HTLC) — ✅ deposit + reserve-tokenisation E2E

The reference stack (`make contracts.deploy-all-with-sync` → `DeployCBWeb3Spoke`)
deploys `FiatCentralBankMoney` (fCeBM) and `HashTimeLockedContract` (HTLC) on the
spoke's Besu chain; the toolkit historically deployed only the Paladin-layer
(Zeto/Pente) + IdentityRegistry, leaving the backend's Besu-signing path dormant.
It is now wired for `environment: local`:

- **`deploy-fiat-token` / `deploy-htlc`** (found) deploy fCeBM + HTLC (signed via the
  KeyProvider), persist `FIAT_TOKEN_ADDRESS` / `HTLC_ADDRESS` to `.deployed-addrs.env`,
  and the join bundle carries both to commercial banks.
- The per-entity backend `.env` renders `BESU_OPERATOR_KEY` (the local public dev
  operator key, exported only by the local emulator), `BESU_CHAIN_ID` (the real
  spoke chain id — 0 makes go-ethereum's `NewKeyedTransactorWithChainID` panic),
  `ENTITY_BESU_ADDRESS` (the escrow proxy's `requester_besu_address`), and
  `PALADIN_IDENTITY` / `CB_PALADIN_IDENTITY` (the escrow proxy's
  `requester_paladin_identity` = Zeto mint recipient). `start-cb-backend` /
  `start-backend` set `PAYMENT_ORCH_BESU_RPC_URL` only when an operator key exists,
  so the path stays OFF outside local (prod wires signing via KMS — FASE 4).
- **Validated E2E (local, real Docker):**
  - Deposit (issuance): bank `POST /payments/deposits` → treasury `approve` +
    `fiat-exchange` → fCeBM minted to the bank wallet; `/token/fiat-balance` reflects it.
  - Reserve tokenisation (escrow): bank `POST /payments/escrows` → treasury `approve`
    → fCeBM burned on Besu + tCeBM minted via Zeto to the bank's Paladin identity
    (cross-node recipient resolution via `ptx_resolveVerifier` works); `/token/balance`
    reflects the minted tCeBM.

Prod (staging) wiring of the operator signing key and per-entity funded wallets is
FASE 4 (not implemented); in local all entities share the public dev operator key,
so per-entity fiat balances are not independently meaningful.

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

## Known issue: `ApproveEscrow` is not atomic — partial settlement on Zeto-mint failure

**Where.** `payment-orchestrator` — `internal/grpc/server/escrow.go`, `ApproveEscrow`
(reserve tokenisation). NOT a toolkit issue; surfaced while validating the escrow
flow end-to-end after the Besu-layer wiring above.

**Symptom.** `ApproveEscrow` performs two on-chain actions in sequence with no
rollback and no idempotency guard:
1. burn fCeBM from the bank's Besu wallet (`s.fiat.Burn`);
2. mint tCeBM to the bank's Paladin identity via Zeto (`s.zeto.Mint`).

If step 2 fails after step 1 commits, the fiat is destroyed but no private token is
minted. Observed live when `requester_paladin_identity` was empty (before the
`PALADIN_IDENTITY` wiring fix): the Zeto mint returned `PD210025: Parameter 'to' is
required`, the backend logged `CRITICAL: fCeBM burned but Zeto mint failed — manual
intervention required`, and 3000 fCeBM was burned with 0 tCeBM minted.

**Aggravating factor — no idempotency guard.** On the mint failure the record is
left `PENDING` (status is only advanced on full success), so a retried `approve`
**burns again** (double-burn) rather than resuming after the burn.

**Impact.** Violates the constitution's atomicity rule ("Partial settlement is
forbidden in production paths; timeout/refund paths must be tested"). Safe to leave
for local demos, but MUST be fixed before any production path.

**Recommended remediation (not yet done):**
- Reorder to mint-then-burn, or make the pair atomic/compensating (burn → on
  mint-failure, re-mint/return the fCeBM), and
- persist an intermediate `BURNED` state so a retry resumes at the mint instead of
  re-burning (idempotency), and
- add a failing test for the burn-ok/mint-fail path before implementing.

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

## Resolved: cross-VM Paladin transport — one-directional routable wiring (fixed 2026-07-17)

**Symptom (before fix).** On the multi-VM LNET lab (CB and bank on different hosts),
`create-pente-context` hung and the orchestrator logged, every ~30s:
```
waiting for Paladin peer funded_operator@spoke-brazil-cb: POST ptx_resolveVerifier:
Post "http://localhost:27645": context deadline exceeded (Client.Timeout exceeded ...)
```
A **transport-level** timeout (not a Paladin RPC error), so the bank's local Paladin
was up but its resolve call blocked server-side. TCP to the CB Paladin `:9000`, the
`extra_hosts` override, and the port publish were all correct — the request leg was fine.

**Root cause: the routable-host wiring was one-directional.** Paladin's reliable
transport is bidirectional — resolving a remote verifier is a request AND a reply, so
BOTH nodes must dial each other's on-chain endpoint. The routable override (feat
c3075acd) gave only the *bank* an `extra_hosts` entry for the CB (request leg). Each
node still registered its **container name** as its endpoint (`dns:///paladin-<node>:9000`),
so the CB, sending the reply, tried to dial `paladin-spoke-brazil-cb1` — which does not
resolve on the CB VM (no reciprocal `extra_hosts`). The reply never returned and the
bank's `ptx_resolveVerifier` timed out. It worked single-host because the shared spoke
Docker network resolves every container name in both directions.

**Fix (Option B — routable endpoint + cert SAN).** When a node's `advertisedHost` is
routable, it now registers `dns:///<advertisedHost>:9000` (its routable host, not the
container name) and carries that host in the transport cert SAN (IP → IPAddresses,
DNS → DNSNames) so the `dns:///` mutual-TLS handshake still validates. Peers on any VM
dial the routable host directly — no per-peer `extra_hosts`. The `PD030011` node-identity
check is on the cert **CN** (= node name), which is unchanged. Gated on `isRoutableHost`,
so single-host keeps the container-name behavior byte-for-byte. See `paladinDialHost` /
`addTransportSAN` and the `gen-tls` / `gen-tls-join` / `register-nodes` /
`register-paladin-node` steps. Because every node now advertises its own routable
host, the bank dials the CB by IP/host and never by container name — so the old
one-directional `extra_hosts` overlay (`paladin-compose.routable.yaml` + `cbPaladinHost`
wiring) became dead code and was removed. Cross-VM peering requires only that
`9000/tcp` be open both directions between each CB↔bank pair.

**Second half — publish gRPC on 9000 (the bank join side).** Advertising `:9000` is only
half the fix: the node must also *listen* there. The routable CB founder already published
its Paladin gRPC on host `9000` (`startPaladinStep`), but the **bank join** step
(`startPaladinJoinStep`) always published in the per-bank `+21000` band (e.g. besu 8645 →
`29645`). So a routable bank registered `dns:///<ip>:9000` while its container was reachable
only on `29645`, and the CB's resolve-reply dial failed with:
```
PD030015: GRPC connection failed for endpoint 'dns:///10.10.0.22:9000':
... dial tcp 10.10.0.22:9000: connect: connection refused
```
Fix: `startPaladinJoinStep` now publishes gRPC on `9000` when the bank's own
`advertisedHost` is routable (mirroring the CB); single-host keeps the `+21000` band
(collision-free when banks share a host). Requires `9000/tcp` open **both** directions
between the CB and each bank VM.
