# API Contracts: Hub Currency Registry

**Feature**: 006-hub-currency-registry  
**Date**: 2026-05-20

---

## New Endpoints

### POST /api/v2/hub/currencies — Register Currency

Registers the caller's currency on the hub `CurrencyRegistry` contract.  
Authenticated as a Central Bank. Authorization enforced on-chain: caller's signer must be `IdentityRegistry.getCentralBankOf(tokenAddress)`.

**Request**

```json
POST /api/v2/hub/currencies
Cookie: access_token=<CB_TOKEN>
Content-Type: application/json

{
  "symbol":        "BRL",
  "country_name":  "Brazil",
  "token_address": "0xa50a51c09a5c451c52bb714527e1974b686d8e77",
  "proposer_cb":   "central_bank_a"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `symbol` | string | yes | Currency symbol (e.g., `"BRL"`). Unique on hub. |
| `country_name` | string | yes | Country name (e.g., `"Brazil"`). |
| `token_address` | string | yes | On-chain tCeBM address. Unique on hub. Caller must be its issuer. |
| `proposer_cb` | string | yes | Human-readable CB identifier (e.g., `"central_bank_a"`). |

**Success Response — 201 Created**

```json
{
  "symbol":     "BRL",
  "tx_hash":    "0xabc123..."
}
```

**Error Responses**

| HTTP | Code | When |
|------|------|------|
| 400 | `INVALID_REQUEST` | Missing required fields or empty strings |
| 409 | `CURRENCY_ALREADY_EXISTS` | Symbol already registered on-chain |
| 409 | `TOKEN_ALREADY_REGISTERED` | `token_address` already registered under a different symbol |
| 403 | `UNAUTHORIZED` | Caller is not the on-chain issuer of `token_address` |
| 500 | `ONCHAIN_ERROR` | Contract revert or RPC failure |

---

### DELETE /api/v2/hub/currencies/:symbol — Remove Currency

Removes the caller's currency from the hub registry. Caller must be the on-chain issuer of the token associated with the given symbol.

**Request**

```
DELETE /api/v2/hub/currencies/BRL
Cookie: access_token=<CB_TOKEN>
```

**Success Response — 200 OK**

```json
{
  "symbol":  "BRL",
  "tx_hash": "0xdef456..."
}
```

**Error Responses**

| HTTP | Code | When |
|------|------|------|
| 404 | `CURRENCY_NOT_FOUND` | No currency registered under this symbol |
| 403 | `UNAUTHORIZED` | Caller is not the on-chain issuer of the token for this symbol |
| 500 | `ONCHAIN_ERROR` | Contract revert or RPC failure |

---

### GET /api/v2/hub/currencies — List All Currencies

Returns all currencies currently registered on the hub. Permissionless — no authentication required.

**Request**

```
GET /api/v2/hub/currencies
```

**Success Response — 200 OK**

```json
{
  "currencies": [
    {
      "symbol":        "BRL",
      "country_name":  "Brazil",
      "token_address": "0xa50a51c09a5c451c52bb714527e1974b686d8e77",
      "proposer_cb":   "central_bank_a"
    },
    {
      "symbol":        "EUR",
      "country_name":  "Germany",
      "token_address": "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e",
      "proposer_cb":   "central_bank_b"
    }
  ]
}
```

**Error Responses**

| HTTP | Code | When |
|------|------|------|
| 500 | `ONCHAIN_ERROR` | RPC failure reading contract |

---

## Existing Endpoint: Semantic Documentation Update

### GET /api/v2/amm/pairs — List All Pairs (documentation update only)

No response schema change. The following semantic aliases are now formally documented:

| Field | Semantic Role | Description |
|-------|--------------|-------------|
| `proposer_cb` | **SideA** | CB that proposed the pair. Authorized by `IdentityRegistry.getCentralBankOf(tokenA)`. |
| `confirmer_cb` | **SideB** | CB that confirmed the pair. Authorized by `IdentityRegistry.getCentralBankOf(tokenB)`. |

**Example response (unchanged structure)**:

```json
{
  "pairs": [
    {
      "pair_id":         "BRL-EUR",
      "status":          "ACTIVE",
      "token_a_address": "0xa50a51c09a5c451c52bb714527e1974b686d8e77",
      "token_b_address": "0x9a3dbca554e9f6b9257aaa24010da8377c57c17e",
      "amm_address":     "0x05d91b9031a655d08e654177336d08543ac4b711",
      "proposer_cb":     "central_bank_a",
      "confirmer_cb":    "central_bank_b",
      "proposed_at":     "2026-05-20T10:00:00Z",
      "confirmed_at":    "2026-05-20T10:05:00Z"
    }
  ]
}
```

---

## Existing Endpoint: Behavior Enhancement

### POST /api/v2/amm/pairs/propose — FR-011 Duplicate Token-Pair Guard

**New behavior**: Before submitting on-chain, the service checks if any existing PROPOSED or ACTIVE pair already exists for the same token combination (order-agnostic). If found, returns:

```json
HTTP 409
{
  "error": "PAIR_ALREADY_EXISTS",
  "code":  "PAIR_ALREADY_EXISTS",
  "detail": "a pair for this token combination already exists with status PROPOSED"
}
```

All other request/response fields unchanged.

---

## Smart Contract Interface (Solidity)

### ICurrencyRegistry.sol

```solidity
// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

interface ICurrencyRegistry {
    struct CurrencyEntry {
        string  symbol;
        string  countryName;
        address tokenAddress;
        string  proposerCB;
    }

    event CurrencyRegistered(
        string  indexed symbol,
        address indexed tokenAddress,
        string  countryName,
        string  proposerCB
    );

    event CurrencyRemoved(
        string  indexed symbol,
        address indexed tokenAddress
    );

    error CurrencyRegistry__AlreadyExists(string symbol);
    error CurrencyRegistry__TokenAlreadyRegistered(address token);
    error CurrencyRegistry__NotFound(string symbol);
    error CurrencyRegistry__Unauthorized();
    error CurrencyRegistry__ZeroAddress();
    error CurrencyRegistry__EmptyString();

    function registerCurrency(
        string  calldata symbol,
        string  calldata countryName,
        address          tokenAddress,
        string  calldata proposerCB
    ) external;

    function removeCurrency(string calldata symbol) external;

    function getCurrency(string calldata symbol)
        external view returns (CurrencyEntry memory);

    function getAllCurrencies()
        external view returns (CurrencyEntry[] memory);
}
```
