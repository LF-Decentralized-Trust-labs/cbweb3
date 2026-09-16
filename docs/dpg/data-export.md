# Extracting and importing data

*DPG Standard, indicator 6 — non-personal data can be exported and imported in
non-proprietary formats.*

Every store in CBWeb3 is readable through an open interface, and no data leaves the
system in a format that requires CBWeb3 to read it back. There are four independent
paths, and an operator needs only one of them.

## 1. The ledger — Ethereum JSON-RPC

Each spoke and the hub is a Hyperledger Besu network. Any node operator reaches the
complete ledger over the **standard Ethereum JSON-RPC API** (HTTP and WebSocket), with no
CBWeb3-specific component in the path:

```bash
# every block, transaction, receipt and log, as JSON
curl -X POST http://<node>:8545 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x1",true],"id":1}'
```

Token balances, issuance and redemption, HTLC locks and releases, AMM swaps and every
contract event are on-chain state, retrievable with `eth_call`, `eth_getLogs` and
`eth_getTransactionReceipt` and decodable with the contract ABIs published in the
repository. Any standard Ethereum client library, indexer or block explorer works against
these endpoints, because nothing about the interface is proprietary to this project.

Zeto private transfers are the deliberate exception: their amounts are commitments, not
plaintext, and are readable only by the counterparties and — through an audited disclosure
path — by the supervisor. That is the privacy property the design exists to provide; see
[Privacy](privacy.md).

## 2. The services — REST and WebSocket, JSON over OpenAPI

The API gateway is specified in
[OpenAPI 3.0.3](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/scenario-a/backend/services/api-gateway/docs/openapi.yaml)
(74 paths) and answers in JSON. Collection endpoints accept filters, so operational data
can be extracted incrementally rather than scraped:

| Data | Endpoint |
|---|---|
| Registered participants and their roles | `GET /api/v1/compliance/participants`, `GET /api/v1/governance/participants` |
| Participant summary by category | `GET /api/v1/compliance/participants/summary` |
| Privileged-action audit trail | `GET /api/v1/compliance/audit/logs`, `GET /api/v1/governance/audit/logs` |
| FX agreement lifecycle and its audit trail | `GET /api/v1/payments/fx/agreements`, `…/{tradeId}/audit` |
| HTLC lock records | `GET /api/v1/htlc/search`, `GET /api/v1/htlc/status/{contractId}` |
| Deposits, escrows and redemptions | `GET /api/v1/payments/deposits`, `…/escrows`, `…/redeems` |
| On-chain governance parameters and circuit-breaker state | `GET /api/v1/governance/parameters`, `…/circuit-breaker/status` |

Paths above are Scenario A. Scenario B is a separate product with its own gateway and its
own [OpenAPI document](https://github.com/LF-Decentralized-Trust-labs/cbweb3/blob/main/platform/scenario-b/backend/services/api-gateway/docs/openapi.yaml),
served under an `/api/v2/` prefix — bridge positions and hub liquidity state, for example,
are read there at `GET /api/v2/bridge/positions`. Read the specification for the scenario
being deployed; do not assume paths carry across.

Because the specification is machine-readable, a client — or an export script — can be
generated for any language with standard OpenAPI tooling. Both gateways are
request/response only: there is no server-sent-events or WebSocket endpoint in either
scenario's backends, so extraction is by polling these collections.

## 3. The databases — PostgreSQL

Service state persists in PostgreSQL. Standard tooling applies, with no CBWeb3 component
involved:

```bash
pg_dump -Fp cbweb3 > cbweb3.sql                       # portable SQL
psql -c "\copy audit_log TO 'audit.csv' CSV HEADER"    # CSV
```

## 4. Specifications and artifacts — files in the repository

Interface contracts, conformance material and reference data are plain text under version
control: OpenAPI YAML in [`Toolbox/contracts/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/Toolbox/contracts),
JSON test vectors and mocks with published JSON Schemas in
[`Toolbox/`](https://github.com/LF-Decentralized-Trust-labs/cbweb3/tree/main/Toolbox), ABIs
and generated contract documentation under `platform/scenario-a/contracts/`. `git clone`
is the export mechanism, and it is complete.

## Importing

The same interfaces run in the other direction. Participants are provisioned through
`POST /api/v1/compliance/participants/provision` and `POST /api/v1/compliance/register`
with JSON bodies; network topology, chain IDs and genesis allocations are declared in YAML
and JSON configuration files consumed by the provisioning toolkit; governance parameters
are set through the governance API; and test data can be loaded from the Toolbox JSON
vectors. No import path requires a proprietary tool or a CBWeb3-specific file format.

## What is deliberately not exportable

Private keys held by the key provider are non-exportable by design, and the plaintext
amounts of Zeto private transfers are not available to non-counterparties outside the
audited disclosure path. Both are security properties, not format lock-in.
