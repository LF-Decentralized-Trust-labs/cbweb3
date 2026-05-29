# Quickstart: Hub Currency Registry

**Feature**: 006-hub-currency-registry  
**Date**: 2026-05-20

---

## Prerequisites

- Hub chain running (Besu / local Hardhat / Anvil on chain ID 1338)
- `IdentityRegistry` deployed and CB addresses registered via `setCentralBankOf(tokenAddress, cbAddress)`
- `CURRENCY_REGISTRY_CONTRACT_ADDRESS` env var set in `.env.infra.central-bank-*.example`
- CB gateway running with `SIGNER_PRIVATE_KEY` matching the CB's on-chain address

---

## 1. Deploy CurrencyRegistry

```bash
cd contracts
forge script script/DeployCurrencyRegistry.s.sol \
  --rpc-url $HUB_RPC_URL \
  --broadcast \
  --sig "run(address)" $IDENTITY_REGISTRY_ADDRESS
```

Note the deployed address and set:
```env
CURRENCY_REGISTRY_CONTRACT_ADDRESS=0x<deployed_address>
```

---

## 2. Register a Currency (CB-A registers BRL)

```bash
curl -s -X POST http://localhost:38080/api/v2/hub/currencies \
  -H "Cookie: access_token=$CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":        "BRL",
    "country_name":  "Brazil",
    "token_address": "0xa50a51c09a5c451c52bb714527e1974b686d8e77",
    "proposer_cb":   "central_bank_a"
  }' | jq .
```

Expected: `201 { "symbol": "BRL", "tx_hash": "0x..." }`

---

## 3. Register a Second Currency (CB-B registers EUR)

```bash
curl -s -X POST http://localhost:48080/api/v2/hub/currencies \
  -H "Cookie: access_token=$CENTRAL_BANK_B_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol":        "EUR",
    "country_name":  "Germany",
    "token_address": "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e",
    "proposer_cb":   "central_bank_b"
  }' | jq .
```

---

## 4. List Registered Currencies

```bash
curl -s http://localhost:38080/api/v2/hub/currencies | jq .
```

Expected: both BRL and EUR in `currencies[]`

---

## 5. Use Registry to Propose a Pair

CB-A reads the EUR token address from the list, then proposes the BRL-EUR pair:

```bash
curl -s -X POST http://localhost:38080/api/v2/amm/pairs/propose \
  -H "Cookie: access_token=$CENTRAL_BANK_A_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pair_id":        "BRL-EUR",
    "token_a_address": "0xa50a51c09a5c451c52bb714527e1974b686d8e77",
    "token_b_address": "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e",
    "amm_address":    "0x05d91b9031a655d08e654177336d08543ac4b711",
    "proposer_cb":    "central_bank_a"
  }' | jq .
```

---

## 6. Remove a Currency (CB-A removes BRL)

```bash
curl -s -X DELETE http://localhost:38080/api/v2/hub/currencies/BRL \
  -H "Cookie: access_token=$CENTRAL_BANK_A_TOKEN" | jq .
```

Expected: `200 { "symbol": "BRL", "tx_hash": "0x..." }`

---

## Environment Variables Required

Add to `.env.infra.central-bank-*.example`:

```env
# CurrencyRegistry — hub currency discovery contract (006-hub-currency-registry)
CURRENCY_REGISTRY_CONTRACT_ADDRESS=
```
