# PvP Settlement — Interface Contract

## Overview

This contract defines the API interface for **Payment-versus-Payment (PvP)** atomic settlement between two central banks exchanging tokenized Central Bank Money (tCeBM).

## Flow

```
Central Bank A (Initiator)              Central Bank B (Counterparty)
        |                                        |
        |── POST /fx/agreement ─────────────────>|  Propose FX deal
        |<── POST /fx/agreement/{id}/accept ─────|  Accept terms
        |                                        |
        |── POST /htlc/lock ───────────────────>|  Lock tCeBM-A (hashLock)
        |<── POST /htlc/lock ───────────────────|  Lock tCeBM-B (same hashLock)
        |                                        |
        |── POST /htlc/settle ─────────────────>|  Reveal secret → claim tCeBM-B
        |<── POST /htlc/settle ─────────────────|  Use secret → claim tCeBM-A
        |                                        |
        |         ✅ Settlement complete           |
```

**Timeout path:** If the secret is not revealed before `timeLock` expires, either party calls `POST /htlc/refund` to reclaim their locked funds.

## Files

| File | Description |
|------|-------------|
| `openapi_pvp_v0.1.0.yaml` | OpenAPI 3.0.3 specification |
| `CHANGELOG.md` | Version history |

## Validation

```bash
# Validate the OpenAPI spec (requires spectral or swagger-cli)
npx @stoplight/spectral-cli lint openapi_pvp_v0.1.0.yaml
```

## Version

Current: **0.1.0** — Initial extraction from CBWeb3 Pilot API (Deliverable 5).
