# PvP Settlement — Reference Mocks

## Purpose

Canonical simulated responses for the PvP settlement flow. These mocks allow integrators to develop and test against deterministic, predictable API responses **without** requiring live blockchain infrastructure.

## Scenario

**Corridor:** Costa Rica (tCeBM-CRC) ↔ Dominican Republic (tCeBM-DOP)

The mock data simulates a complete PvP settlement between two central banks:
- **CB-CostaRica** (initiator): `0xCB001_CostaRica`
- **CB-DominicanRepublic** (counterparty): `0xCB002_DominicanRepublic`

## Files

| File | Description |
|------|-------------|
| `happy-path/` | Full successful PvP settlement (6 steps) |
| `timeout-refund/` | HTLC expiration and refund path |

## How to use

Each mock file contains:
- `request`: the HTTP request (method, path, headers, body)
- `response`: the expected HTTP response (status, headers, body)

These files can be used:
1. **Manually** — as reference for integrators building clients
2. **With Prism** — as an OpenAPI mock server: `npx @stoplight/prism-cli mock ../contracts/pvp/openapi_pvp_v0.1.0.yaml`
3. **With Postman** — import and configure mock server responses

## Synthetic data disclaimer

All data in these mocks is **synthetic**. Addresses, amounts, hashes, and identifiers are fabricated for testing. No real financial data or credentials are included.
