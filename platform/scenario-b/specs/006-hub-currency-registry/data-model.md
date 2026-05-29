# Data Model: Hub Currency Registry

**Feature**: 006-hub-currency-registry  
**Date**: 2026-05-20

---

## On-Chain Entities (Solidity — CurrencyRegistry.sol)

### CurrencyEntry

Represents a registered currency on the international hub. Stored in `CurrencyRegistry.sol`.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `symbol` | `string` | **PK**, unique, non-empty | ISO-style currency symbol (e.g., `"BRL"`, `"EUR"`). Used as the primary lookup key. |
| `countryName` | `string` | non-empty | Human-readable country name (e.g., `"Brazil"`, `"Germany"`). |
| `tokenAddress` | `address` | **Unique**, non-zero | On-chain address of the tCeBM token on the hub. Must be the token for which `IdentityRegistry.getCentralBankOf(tokenAddress) == msg.sender`. |
| `proposerCB` | `string` | non-empty | Human-readable Central Bank identifier (e.g., `"central_bank_a"`). Free-form label, not validated. |

**Uniqueness constraints**:
- `symbol` is a globally unique key — no two entries may share the same symbol.
- `tokenAddress` is a globally unique key — no two entries may share the same token address, even under different symbols.

**Authorization**:
- `registerCurrency`: caller must be `IdentityRegistry.getCentralBankOf(tokenAddress)`.
- `removeCurrency(symbol)`: caller must be `IdentityRegistry.getCentralBankOf(entry.tokenAddress)` for the entry identified by `symbol`.

**Lifecycle**:
```
Not Registered
     │ registerCurrency(symbol, countryName, tokenAddress, proposerCB)
     ▼
  Registered
     │ removeCurrency(symbol)
     ▼
Not Registered
```

---

### On-Chain Storage Layout (CurrencyRegistry.sol)

```
IIdentityRegistry                 REGISTRY           (immutable)
mapping(bytes32 => CurrencyEntry) _bySymbolKey        keccak256(symbol) → entry
mapping(address => bytes32)       _tokenToSymbolKey   tokenAddress → symbolKey
mapping(bytes32 => bool)          _symbolExists       existence guard
string[]                          _symbols            ordered list for iteration
```

**Key derivation**: `_key(symbol) = keccak256(bytes(symbol))`

**Tombstone on remove**: `_symbolExists[key] = false` + `delete _tokenToSymbolKey[entry.tokenAddress]`.  
Entry remains in `_symbols` but is skipped during `getAllCurrencies()` iteration.

---

### Events

| Event | Arguments | When emitted |
|-------|-----------|--------------|
| `CurrencyRegistered` | `symbol` (indexed), `tokenAddress` (indexed), `countryName`, `proposerCB` | On successful `registerCurrency` |
| `CurrencyRemoved` | `symbol` (indexed), `tokenAddress` (indexed) | On successful `removeCurrency` |

### Errors

| Error | When thrown |
|-------|-------------|
| `CurrencyRegistry__AlreadyExists(string symbol)` | `registerCurrency` called with a symbol already registered |
| `CurrencyRegistry__TokenAlreadyRegistered(address token)` | `registerCurrency` called with a tokenAddress already registered under any symbol |
| `CurrencyRegistry__NotFound(string symbol)` | `removeCurrency` or `getCurrency` called for an unknown symbol |
| `CurrencyRegistry__Unauthorized()` | Caller is not `getCentralBankOf(tokenAddress)` |
| `CurrencyRegistry__ZeroAddress()` | `tokenAddress == address(0)` |
| `CurrencyRegistry__EmptyString()` | `symbol` or `countryName` or `proposerCB` is empty |

---

## Off-Chain Entities (Go — api-gateway)

### domain.CurrencyEntry (Go struct)

In-memory representation of a `CurrencyEntry` returned by the contract.

```go
type CurrencyEntry struct {
    Symbol      string // e.g., "BRL"
    CountryName string // e.g., "Brazil"
    TokenAddress string // e.g., "0xa50a51..."
    ProposerCB  string // e.g., "central_bank_a"
}
```

No DB table — read directly from on-chain via `getAllCurrencies()`. The contract is the source of truth.

---

## Existing Entity: PairProposal (Enhanced Documentation)

No schema changes. The existing `pair_proposals` PostgreSQL table is unchanged. The following semantic mapping is formalized:

| DB Column | SideA/SideB Role | Description |
|-----------|-----------------|-------------|
| `proposer_cb` | **SideA** | CB that proposed the pair (`POST /api/v2/amm/pairs/propose`) |
| `confirmer_cb` | **SideB** | CB that confirmed the pair (`POST /api/v2/amm/pairs/confirm`) |

### FR-011: Token-Pair Uniqueness Index

A new query method is added to the `PairRepository` to enforce uniqueness by token combination (order-agnostic):

```sql
SELECT * FROM pair_proposals
WHERE (
    (token_a_address = $1 AND token_b_address = $2)
 OR (token_a_address = $2 AND token_b_address = $1)
)
AND status IN ('PROPOSED', 'ACTIVE')
LIMIT 1;
```

This check runs in `PairService.ProposePair` before submitting on-chain. No new columns or indexes are required beyond potentially adding a composite GIN/B-tree index on `(token_a_address, token_b_address)` for performance (deferred — the table will remain small).

---

## Entity Relationship Summary

```
IdentityRegistry.getCentralBankOf(tokenAddress) ──► CB on-chain address
                                                          │
                        ┌─────────────────────────────────┤
                        │                                 │
                        ▼                                 ▼
              CurrencyEntry                         PairProposal
          (on-chain, CurrencyRegistry)          (off-chain, pair_proposals)
          ─────────────────────────────        ─────────────────────────────
          symbol (PK unique)                  pair_id (PK)
          countryName                         token_a_address ──► CurrencyEntry.tokenAddress
          tokenAddress (unique)               token_b_address ──► CurrencyEntry.tokenAddress
          proposerCB                          proposer_cb (= SideA)
                                              confirmer_cb (= SideB)
                                              status, amm_address, …
```
