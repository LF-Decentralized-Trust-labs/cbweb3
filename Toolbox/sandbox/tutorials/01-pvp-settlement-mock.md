# Tutorial 1: PvP Settlement against Mock Server

Execute a complete cross-border PvP settlement flow using the Prism mock server. No real infrastructure required.

**Time:** ~15 minutes
**Prerequisites:** Node.js 18+, curl (see [prerequisites.md](../devnet-guide/prerequisites.md))

---

## Background

You will simulate a foreign exchange settlement between two central banks:
- **Country A** sends 1,000,000 tCeBM-A
- **Country B** sends 58,000 tCeBM-B
- The exchange is atomic: both sides settle or neither does

For a detailed explanation of each step, see [flow-walkthrough.md](../devnet-guide/flow-walkthrough.md).

---

## Setup

### Start the mock server

Open a terminal and run:

```bash
npx @stoplight/prism-cli mock Toolbox/contracts/pvp/openapi_pvp_v0.1.0.yaml --port 4010
```

Wait until you see `Prism is listening on http://127.0.0.1:4010`.

Keep this terminal open. Open a **second terminal** for the API calls below.

> **Note on URL paths:** The OpenAPI spec defines a server base URL of `/api/v1` and relative paths like `/fx/agreement`. Prism serves the paths directly (without the server prefix), so the URLs below use `http://localhost:4010/fx/agreement` — not `/api/v1/fx/agreement`. When targeting a real implementation, use the full path including `/api/v1/`.

---

## The 6-step settlement

### Step 1: Create FX Agreement

Country A proposes an FX deal to Country B.

```bash
curl -s -X POST http://localhost:4010/fx/agreement \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB001" \
  -d '{
    "sourceCurrency": "tCeBM-A",
    "targetCurrency": "tCeBM-B",
    "sourceAmount": "1000000",
    "exchangeRate": "0.058",
    "counterpartyAddress": "0xCB002_CountryB"
  }'
```

**Expected response (201 Created):**
```json
{
  "agreementId": "agr-2026-001",
  "status": "PROPOSED"
}
```

The agreement is created with status `PROPOSED`. Country A has proposed terms; Country B has not yet accepted.

**Corresponding mock:** `mocks/pvp/happy-path/01_create_fx_agreement.json`

---

### Step 2: Accept FX Agreement

Country B accepts the proposed terms.

```bash
curl -s -X POST http://localhost:4010/fx/agreement/agr-2026-001/accept \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB002"
```

**Expected response (200 OK):**
```json
{
  "agreementId": "agr-2026-001",
  "status": "READY_FOR_SETTLEMENT"
}
```

The agreement is now `READY_FOR_SETTLEMENT`. Both parties are committed. The HTLC locking phase can begin.

**Corresponding mock:** `mocks/pvp/happy-path/02_accept_fx_agreement.json`

---

### Step 3: Lock A (initiator)

Country A locks 1,000,000 tCeBM-A in an HTLC smart contract.

The `hashLock` is the SHA-256 hash of the secret `cbweb3-test-secret-2026`. Only someone who knows the secret can unlock the funds.

```bash
curl -s -X POST http://localhost:4010/htlc/lock \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB001" \
  -d '{
    "agreementId": "agr-2026-001",
    "hashLock": "0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069",
    "timeLock": 1740000000
  }'
```

**Expected response (201 Created):**
```json
{
  "contractId": "htlc-2026-001a",
  "transactionHash": "0xabc123...def456",
  "status": "LOCKED",
  "blockNumber": 18500001
}
```

The funds are now escrowed on-chain. Country A cannot spend them, but the counterparty cannot claim them without the secret.

**Corresponding mock:** `mocks/pvp/happy-path/03_htlc_lock_initiator.json`

---

### Step 4: Lock B (counterparty)

Country B locks 58,000 tCeBM-B using the **same hashLock**.

In a real system, Country B would verify (via the Cacti interoperability layer) that Country A's funds are locked before proceeding. In this tutorial, we proceed directly.

```bash
curl -s -X POST http://localhost:4010/htlc/lock \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB002" \
  -d '{
    "agreementId": "agr-2026-001",
    "hashLock": "0x7f83b1657ff1fc53b92dc18148a1d65dfc2d4b1fa3d677284addd200126d9069",
    "timeLock": 1739996400
  }'
```

**Expected response (201 Created):**
```json
{
  "contractId": "htlc-2026-001a",
  "transactionHash": "0xabc123...def456",
  "status": "LOCKED",
  "blockNumber": 18500001
}
```

Both sides now have funds locked. Notice the counterparty's `timeLock` (1739996400) is earlier than the initiator's (1740000000) — this gives the initiator time to settle after the secret is revealed.

> **Note:** Prism is stateless and returns the same example for all `/htlc/lock` calls. In a real implementation, each lock would return a different `contractId`. The standalone mock files in `mocks/pvp/happy-path/` show the distinct values (`htlc-2026-001a` and `htlc-2026-001b`).

**Corresponding mock:** `mocks/pvp/happy-path/04_htlc_lock_counterparty.json`

---

### Step 5: Settle B (initiator reveals secret)

Country A reveals the secret to claim the 58,000 tCeBM-B locked by Country B.

This is the **critical moment**: revealing the secret exposes it on-chain, enabling the counterparty to settle the other leg.

```bash
curl -s -X POST http://localhost:4010/htlc/settle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB001" \
  -d '{
    "contractId": "htlc-2026-001b",
    "secret": "cbweb3-test-secret-2026"
  }'
```

**Expected response (200 OK):**
```json
{
  "contractId": "htlc-2026-001a",
  "transactionHash": "0xdef789...abc012",
  "status": "SETTLED",
  "blockNumber": 18500010
}
```

Country A has claimed the tCeBM-B. The secret `cbweb3-test-secret-2026` is now visible on the blockchain.

**Corresponding mock:** `mocks/pvp/happy-path/05_htlc_settle_counterparty.json`

---

### Step 6: Settle A (counterparty uses revealed secret)

Country B reads the revealed secret from on-chain and uses it to claim the 1,000,000 tCeBM-A.

```bash
curl -s -X POST http://localhost:4010/htlc/settle \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.SYNTHETIC_TOKEN_CB002" \
  -d '{
    "contractId": "htlc-2026-001a",
    "secret": "cbweb3-test-secret-2026"
  }'
```

**Expected response (200 OK):**
```json
{
  "contractId": "htlc-2026-001a",
  "transactionHash": "0xdef789...abc012",
  "status": "SETTLED",
  "blockNumber": 18500010
}
```

**Settlement complete.** Both central banks have received their funds:
- Country A received 58,000 tCeBM-B
- Country B received 1,000,000 tCeBM-A

**Corresponding mock:** `mocks/pvp/happy-path/06_htlc_settle_initiator.json`

---

## Important notes about the mock server

1. **Prism is stateless.** It returns the OpenAPI example for each endpoint regardless of what you send. In a real implementation, step 2 would require a valid `agreementId` from step 1.

2. **All data is synthetic.** The addresses, tokens, hashes, and amounts are fabricated for testing. See the [Toolbox README](../../README.md#synthetic-data-policy).

3. **Error responses.** To test error paths, use the `Prefer` header:
   ```bash
   # Get a 400 Bad Request
   curl -s -X POST http://localhost:4010/htlc/settle \
     -H "Prefer: code=400" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer test" \
     -d '{"contractId":"test","secret":"wrong"}' | jq .
   ```

---

## Next steps

- **Run automated tests:** [Tutorial 2: Validate your implementation](02-validate-implementation.md)
- **Explore error cases:** Review `test-vectors/pvp/pvp_htlc_vectors.json` for all edge cases
- **Understand the architecture:** [Architecture Overview](../devnet-guide/architecture-overview.md)
