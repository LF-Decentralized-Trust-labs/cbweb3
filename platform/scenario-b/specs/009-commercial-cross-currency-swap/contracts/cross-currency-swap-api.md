# API Contract: Commercial Cross-Currency Swap

**Feature**: 009-commercial-cross-currency-swap  
**Phase**: Phase 1 - API Interface Design  
**Date**: 2026-05-26

## Overview

Este documento define os contratos HTTP (request/response schemas) para os endpoints de swap cross-currency. Agnóstico de implementação (não especifica framework HTTP, serialization format details, etc).

---

## POST /api/v2/amm/swap/cross-currency

**Purpose**: Executar swap cross-currency end-to-end (bridge-in → swap Hub → bridge-out)

**Authentication**: Required (JWT Bearer token)  
**Authorization**: Role `commercial_bank` required  
**Rate Limiting**: 10 requests/min, 100 requests/hour per `payer_bank_id`

### Request

**HTTP Method**: `POST`  
**Content-Type**: `application/json`

#### Request Body Schema

```json
{
  "source_currency": "string",      // Required: "BRL", "ARS", etc (Spoke-A asset)
  "target_currency": "string",      // Required: "ARS", "BRL", etc (Spoke-B asset)
  "amount_out": "string",           // Required: Desired amount in target currency (wei string)
  "max_amount_in": "string",        // Required: Maximum willing to pay in source currency (slippage protection, wei string)
  "payer_bank_id": "string",        // Required: Bank ID of payer (commercial bank initiating swap)
  "beneficiary_bank_id": "string",  // Required: Bank ID of beneficiary (receiver in Spoke-B)
  "quote_id": "string"              // Optional: Quote ID from prior GET /quote call (for validation)
}
```

#### Field Constraints

- `source_currency`, `target_currency`: Must be valid currency symbols registered in system (e.g., "BRL", "ARS")
- `amount_out`: Must be positive integer string in wei (e.g., "1000000000000000000" = 1 token)
- `max_amount_in`: Must be positive integer string in wei, ≥ expected `amount_in` from quote
- `payer_bank_id`: Must match JWT claim `bank_id` (cannot swap on behalf of another bank)
- `beneficiary_bank_id`: Must exist in system (registered commercial bank or central bank)
- `quote_id`: If provided, must reference a valid non-expired quote from `/quote` endpoint

#### Example Request

```json
{
  "source_currency": "BRL",
  "target_currency": "ARS",
  "amount_out": "2000000000000000000000",
  "max_amount_in": "1050000000000000000000",
  "payer_bank_id": "bank-a",
  "beneficiary_bank_id": "bank-d",
  "quote_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Response

#### Success Response (HTTP 200)

```json
{
  "swap_id": "string",                    // UUID of CrossCurrencySwapOperation
  "correlation_id": "string",             // UUID for tracing across 3 operations
  "status": "string",                     // Current status (initially "BRIDGE_IN_PROGRESS")
  "amount_in": "string",                  // Actual amount paid in source currency (wei)
  "amount_out": "string",                 // Amount received in target currency (wei)
  "effective_rate": number,               // amount_out / amount_in (actual rate achieved)
  "bridge_in_position_id": "string",      // UUID of bridge position (Spoke-A → Hub)
  "swap_tx_hash": "string",               // Transaction hash of swap on Hub AMM (null initially)
  "bridge_out_position_id": "string",     // UUID of bridge position (Hub → Spoke-B, null initially)
  "created_at": "string"                  // ISO 8601 timestamp
}
```

#### Error Responses

**HTTP 400 - Bad Request**
```json
{
  "error": "invalid request body",
  "error_code": "INVALID_REQUEST",
  "details": "amount_out must be positive integer string"
}
```

**HTTP 401 - Unauthorized**
```json
{
  "error": "missing or invalid JWT token",
  "error_code": "UNAUTHORIZED"
}
```

**HTTP 403 - Forbidden**
```json
{
  "error": "payer_bank_id does not match authenticated bank",
  "error_code": "FORBIDDEN"
}
```

**HTTP 422 - Unprocessable Entity** (Business logic errors)

Pool Not Active:
```json
{
  "error": "pool W-BRL-ARS is not active (status: EMPTY)",
  "error_code": "POOL_NOT_ACTIVE",
  "recommended_action": "Wait for central banks to provision liquidity"
}
```

Circuit Breaker Halted:
```json
{
  "error": "pool W-BRL-ARS is halted by circuit breaker",
  "error_code": "CIRCUIT_BREAKER_HALTED",
  "recommended_action": "Wait for governance to resume pool",
  "halted_by": "central-bank-a",
  "halted_at": "2026-05-26T15:30:00Z"
}
```

Slippage Limit Exceeded:
```json
{
  "error": "required amount_in (1080 BRL) exceeds max_amount_in (1050 BRL)",
  "error_code": "SLIPPAGE_LIMIT_EXCEEDED",
  "recommended_action": "Increase max_amount_in or obtain new quote",
  "required_amount_in": "1080000000000000000000",
  "max_amount_in": "1050000000000000000000"
}
```

Insufficient Pool Liquidity:
```json
{
  "error": "pool has insufficient reserve_b to fulfill amount_out",
  "error_code": "INSUFFICIENT_POOL_LIQUIDITY",
  "recommended_action": "Reduce amount_out or wait for liquidity provision",
  "requested_amount_out": "250000000000000000000000",
  "available_reserve_b": "200000000000000000000000"
}
```

Quote Expired:
```json
{
  "error": "quote 550e8400-e29b-41d4-a716-446655440000 expired at 2026-05-26T15:00:15Z",
  "error_code": "QUOTE_EXPIRED",
  "recommended_action": "Obtain new quote via GET /api/v2/amm/quote/exact-output",
  "quote_id": "550e8400-e29b-41d4-a716-446655440000",
  "expired_at": "2026-05-26T15:00:15Z"
}
```

**HTTP 429 - Too Many Requests** (Rate limit exceeded)
```json
{
  "error": "rate limit exceeded: 10 swaps per minute",
  "error_code": "RATE_LIMIT_EXCEEDED",
  "retry_after": 45,
  "limit": 10,
  "window": "minute"
}
```

**HTTP 500 - Internal Server Error**
```json
{
  "error": "swap execution failed",
  "error_code": "INTERNAL_ERROR"
}
```

---

## GET /api/v2/amm/quote/exact-output

**Purpose**: Obter cotação de swap com timestamp de validade (15s TTL)

**Changes from existing**: Adiciona campos `created_at`, `valid_until`, `time_remaining_seconds`

**Authentication**: Required (JWT Bearer token)  
**Authorization**: Role `commercial_bank` required

### Request

**HTTP Method**: `GET`  
**Query Parameters**:
- `pool_pair` (required): Pool pair ID, e.g., "W-BRL-ARS"
- `amount_out` (required): Desired output amount in wei string
- `max_slippage_pct` (optional): Maximum slippage tolerance as decimal (default: 0.01 = 1%)

#### Example Request

```
GET /api/v2/amm/quote/exact-output?pool_pair=W-BRL-ARS&amount_out=2000000000000000000000&max_slippage_pct=0.01
```

### Response

#### Success Response (HTTP 200)

```json
{
  "quote_id": "string",             // UUID of quote (for validation in swap request)
  "pool_pair": "string",            // "W-BRL-ARS"
  "amount_out": "string",           // Requested output amount (wei)
  "amount_in": "string",            // Required input amount (wei, includes fee)
  "effective_rate": number,         // amount_out / amount_in
  "fee_bps": number,                // Pool fee in basis points (e.g., 30 = 0.3%)
  "max_slippage_pct": number,       // User-provided slippage tolerance
  "reserve_a_snapshot": "string",   // Current reserve A (wei, for audit)
  "reserve_b_snapshot": "string",   // Current reserve B (wei, for audit)
  "created_at": "string",           // ISO 8601 timestamp (server UTC)
  "valid_until": "string",          // ISO 8601 timestamp (created_at + 15s)
  "time_remaining_seconds": number  // Computed: valid_until - now (countdown for frontend)
}
```

#### Error Responses

Similar to `/cross-currency` endpoint: 400 (bad params), 401 (unauthorized), 422 (pool not active, insufficient liquidity), 500 (internal error)

---

## GET /api/v2/amm/swap/cross-currency/:swap_id

**Purpose**: Obter status de operação de swap cross-currency em andamento

**Authentication**: Required (JWT Bearer token)  
**Authorization**: Must be `payer_bank_id` or `beneficiary_bank_id` of the swap

### Request

**HTTP Method**: `GET`  
**Path Parameter**: `swap_id` (UUID)

#### Example Request

```
GET /api/v2/amm/swap/cross-currency/7a3d2f1e-8b4c-4d5e-9f6a-1b2c3d4e5f67
```

### Response

#### Success Response (HTTP 200)

```json
{
  "swap_id": "string",
  "correlation_id": "string",
  "status": "string",                     // QUOTING | BRIDGE_IN_PROGRESS | SWAP_IN_PROGRESS | BRIDGE_OUT_PROGRESS | COMPLETED | FAILED
  "payer_bank_id": "string",
  "beneficiary_bank_id": "string",
  "source_currency": "string",
  "target_currency": "string",
  "amount_in": "string",
  "amount_out": "string",
  "effective_rate": number,
  "bridge_in_position_id": "string",
  "bridge_in_status": "string",           // LOCKING | ACTIVE | BURNED (from BridgePosition)
  "swap_tx_hash": "string",
  "swap_confirmations": number,           // On-chain confirmations (0 if pending)
  "bridge_out_position_id": "string",
  "bridge_out_status": "string",          // BURNING | UNLOCKED (from BridgePosition)
  "failure_reason": "string",             // Null unless status=FAILED
  "created_at": "string",
  "completed_at": "string",               // Null unless status=COMPLETED|FAILED
  "duration_seconds": number              // Computed: completed_at - created_at (latency)
}
```

#### Error Responses

**HTTP 404 - Not Found**
```json
{
  "error": "swap operation not found",
  "error_code": "NOT_FOUND",
  "swap_id": "7a3d2f1e-8b4c-4d5e-9f6a-1b2c3d4e5f67"
}
```

**HTTP 403 - Forbidden**
```json
{
  "error": "not authorized to view this swap",
  "error_code": "FORBIDDEN"
}
```

---

## GET /api/v2/amm/pool/:pair/circuit-breaker-status

**Purpose**: Obter status do circuit breaker de um pool (OK vs HALTED)

**Authentication**: Required (JWT Bearer token)  
**Authorization**: Any authenticated user

### Request

**HTTP Method**: `GET`  
**Path Parameter**: `pair` (string, e.g., "W-BRL-ARS")

#### Example Request

```
GET /api/v2/amm/pool/W-BRL-ARS/circuit-breaker-status
```

### Response

#### Success Response (HTTP 200)

```json
{
  "pool_pair": "string",
  "status": "string",                // "OK" | "HALTED"
  "halted_by": "string",             // Bank ID of CB who halted (null if status=OK)
  "halted_at": "string",             // ISO 8601 timestamp (null if status=OK)
  "reason": "string",                // Governance reason for halt (null if status=OK)
  "last_checked_at": "string"        // ISO 8601 timestamp (cache freshness indicator)
}
```

#### Example Responses

Pool OK:
```json
{
  "pool_pair": "W-BRL-ARS",
  "status": "OK",
  "halted_by": null,
  "halted_at": null,
  "reason": null,
  "last_checked_at": "2026-05-26T15:00:00Z"
}
```

Pool Halted:
```json
{
  "pool_pair": "W-BRL-ARS",
  "status": "HALTED",
  "halted_by": "central-bank-a",
  "halted_at": "2026-05-26T14:30:00Z",
  "reason": "Regulatory review pending",
  "last_checked_at": "2026-05-26T15:00:00Z"
}
```

---

## Polling Behavior Recommendations

Frontend deve implementar polling para acompanhar progresso do swap:

### Phase 1: Bridge-In Monitoring
- **Endpoint**: `GET /api/v2/bridge/positions?state=ACTIVE`
- **Interval**: 5 segundos
- **Timeout**: 120 segundos
- **Success Criteria**: `bridge_state=ACTIVE` para `bridge_in_position_id`

### Phase 2: Swap Monitoring
- **Endpoint**: `GET /api/v2/amm/swap/cross-currency/:swap_id`
- **Interval**: 3 segundos
- **Timeout**: 60 segundos (aviso após 30s se ainda SWAP_IN_PROGRESS)
- **Success Criteria**: `status=BRIDGE_OUT_PROGRESS` (swap tx confirmed on-chain)

### Phase 3: Bridge-Out Monitoring
- **Endpoint**: `GET /api/v2/bridge/positions?state=UNLOCKED`
- **Interval**: 5 segundos
- **Timeout**: 120 segundos
- **Success Criteria**: `bridge_state=UNLOCKED` para `bridge_out_position_id`

### Overall Timeout
- **Total E2E Timeout**: 180 segundos (3 minutos)
- Se timeout → mostrar mensagem "Operação em andamento - pode levar mais tempo que o habitual. Verifique status via Histórico de Operações"

---

## Error Code Summary Table

| Error Code | HTTP Status | Meaning | Recommended User Action |
|------------|-------------|---------|------------------------|
| `INVALID_REQUEST` | 400 | Malformed request body or missing required fields | Fix request parameters |
| `UNAUTHORIZED` | 401 | Missing or invalid JWT token | Re-authenticate |
| `FORBIDDEN` | 403 | User not allowed to perform action | Contact administrator |
| `NOT_FOUND` | 404 | Resource (swap, pool) not found | Verify resource ID |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many swap requests in time window | Wait `retry_after` seconds |
| `POOL_NOT_ACTIVE` | 422 | Pool is EMPTY or PENDING_COUNTERPART | Wait for CB liquidity provision |
| `CIRCUIT_BREAKER_HALTED` | 422 | Pool paused by governance | Wait for governance approval |
| `SLIPPAGE_LIMIT_EXCEEDED` | 422 | Price moved adversely beyond tolerance | Increase max_amount_in or refresh quote |
| `INSUFFICIENT_POOL_LIQUIDITY` | 422 | Pool reserves too low for requested amount | Reduce amount_out or wait for liquidity |
| `QUOTE_EXPIRED` | 422 | Quote older than 15 seconds | Obtain new quote |
| `INTERNAL_ERROR` | 500 | Unexpected server error | Retry or contact support |

---

## Backward Compatibility

- **Existing endpoints unchanged**: `GET /api/v2/amm/quote/exact-output` retains all existing fields, apenas adiciona `created_at`, `valid_until`, `time_remaining_seconds`
- **New endpoints**: `POST /api/v2/amm/swap/cross-currency` e `GET /api/v2/amm/swap/cross-currency/:swap_id` são novos, não afetam fluxos existentes (CBs usam `/liquidity/commit` diretamente)

---

## Next Steps

Com API contracts definidos, prosseguir para:
- **quickstart.md**: Tryout manual E2E com exemplos de cURL
- **tasks.md**: Decompor implementação em tarefas (via `/speckit.tasks`)
