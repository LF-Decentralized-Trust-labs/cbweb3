# Handoff: Cooperative Liquidity Wizard Endpoint Test

Date: 2026-06-09  
Branch: `001-hub-network-isolation`  
Repo: `/Users/venzi/repos/LACNetNetworks/cbweb3-platform`  
Working directory used for live checks: `scenario-b/`

## Context

The full Scenario B stack was already running locally:

- Governance portals:
  - CB-A: `http://localhost:5177`
  - CB-B: `http://localhost:5178`
- API gateways:
  - CB-A: `http://localhost:38080`
  - CB-B: `http://localhost:60080`
- Hub RPC: `http://localhost:8845`, chain `1337`
- Spoke-A RPC: `http://localhost:8645`, chain `1338`
- Spoke-B RPC: `http://localhost:8745`, chain `1339`

The user was having trouble with the Cooperative Liquidity Wizard. The wizard was tested by calling the same backend endpoints the governance frontend uses.

## Wizard Endpoint Sequence

Frontend source:

- `scenario-b/frontend/apps/governance/src/services/api/liquidity.api.ts`
- `scenario-b/frontend/apps/governance/src/features/liquidity/liquidity.store.ts`

Sequence:

1. Login via `/api/v1/auth/login` and retain the `access_token` cookie.
2. `POST /api/v2/bridge/lock-mint` with `{"amount":"..."}`.
3. Poll `GET /api/v2/bridge/positions?state=ACTIVE`.
4. `POST /api/v2/amm/liquidity/commit` with `{"pool_pair":"W-BRL-ARS","amount":"..."}`.
5. Poll:
   - `GET /api/v2/amm/liquidity/commits?pool_pair=W-BRL-ARS`
   - `GET /api/v2/amm/pool/W-BRL-ARS/status`
6. Final check:
   - `GET /api/v2/amm/liquidity/positions?pool_pair=W-BRL-ARS&provider_id=<central-bank-id>`

## Test Run Summary

Amount used for the direct endpoint test:

```text
1000000000000000000
```

Pool pair:

```text
W-BRL-ARS
```

Authentication:

- CB-A login returned `200 OK`.
- CB-B login returned `200 OK`.
- CB-A `/api/v1/auth/me` returned `bankId: central-bank-a`, wallet `0x627306090abaB3A6e1400e9345bC60c78a8BEf57`.
- CB-B `/api/v1/auth/me` returned `bankId: central-bank-b`, but wallet was also `0x627306090abaB3A6e1400e9345bC60c78a8BEf57`.

That CB-B wallet is suspicious because CB-B hub signer is configured as:

```text
0xf17f52151EbEF6C7334FAD080c5704D77216b732
```

## Observed Results

### Initial Pool State

CB-A pool status:

- `pool_status`: `PENDING_COUNTERPART`
- `reserve_a`: `0`
- `reserve_b`: `0`
- pending commit:
  - `commit_id`: `55b6e1ab-250b-4eb6-8f85-b0baef8d872d`
  - `provider_id`: `central-bank-a`
  - `side`: `A`
  - `amount`: `1000`
  - `status`: `PENDING`

CB-B pool status:

- `pool_status`: `EMPTY`
- no pending commits in CB-B local DB.

### CB-A Flow

CB-A `POST /api/v2/bridge/lock-mint` returned `201 Created`.

Created position:

```text
3404b585-2b17-4346-9be8-c9a4a4b87bcb
```

DB state in `cbweb3_central_bank_a.bridged_asset_positions`:

- `bridge_state`: `ACTIVE`
- `owner_bank_id`: `central-bank-a`
- `spoke_network`: `spoke-a`
- `native_asset`: `tCeBM_BRL`
- `mirrored_asset`: `0xecfcab0a285d3380e488a39b4bb21e777f8a4eac`
- `mirrored_amount`: `1000000000000000000`

CB-A payment-orchestrator log:

```text
lock-mint ok — positionID=3404b585-2b17-4346-9be8-c9a4a4b87bcb token=0xecfcab0a285d3380e488a39b4bb21e777f8a4eac amount=1000000000000000000 recipient=0x627306090abaB3A6e1400e9345bC60c78a8BEf57 minter=0x627306090abaB3A6e1400e9345bC60c78a8BEf57
```

CB-A `POST /api/v2/amm/liquidity/commit` with amount `1000000000000000000` failed:

```json
{"code":"INSUFFICIENT_BALANCE","error":"insufficient W-tCeBM balance — lock-mint at least the commit amount before committing"}
```

Important note: CB-A already had an older pending commit for amount `1000`. The fresh `1e18` bridge position is ACTIVE, but the commit did not proceed.

### CB-B Flow

CB-B `POST /api/v2/bridge/lock-mint` returned `201 Created`.

Created position:

```text
d68a1b73-8615-4a0b-adb7-6a2b0ff36e23
```

The wizard-style poll of `GET /api/v2/bridge/positions?state=ACTIVE` returned no ACTIVE positions for the full polling window.

CB-B `POST /api/v2/amm/liquidity/commit` then failed:

```json
{"code":"BRIDGE_POSITION_NOT_ACTIVE","error":"no active bridge position found for this side — wait for Relayer confirmation"}
```

DB state in `cbweb3_central_bank_b.bridged_asset_positions`:

- `bridge_state`: `RECONCILIATION_REQUIRED`
- `owner_bank_id`: `central-bank-b`
- `spoke_network`: `spoke-b`
- `native_asset`: `0xf12b5dd4ead5f743c6baa640b0216200e89b60da`
- `mirrored_asset`: `0x38cf23c52bb4b13f051aec09580a2de845a7fa35`
- `mirrored_amount`: `1000000000000000000`

CB-B payment-orchestrator log:

```text
RelayerWorker item 361964bc-177d-45d7-bbb5-59471338a0f2 exhausted after 5 attempts: spoke lock (position=d68a1b73-8615-4a0b-adb7-6a2b0ff36e23): transaction reverted on-chain (tx=0xa482be71352ca2fe30d5f9f6888f998d32ed058f99947ccbbbaee7f57f46f3ee) — check contract permissions and token allowances
```

CB-B has zero commits:

```text
select * from pool_commits order by created_at desc limit 10;
-- 0 rows
```

## On-Chain Checks

Checked Spoke-B with `cast` against `http://localhost:8745`.

Relevant addresses:

- Actual payment-orchestrator signer derived from `SIGNER_PRIVATE_KEY`: `0x627306090abaB3A6e1400e9345bC60c78a8BEf57`
- CB-B hub signer derived from `CENTRAL_BANK_B_PRIVATE_KEY`: `0xf17f52151EbEF6C7334FAD080c5704D77216b732`
- Spoke-B token: `0xf12b5dd4ead5f743c6baa640b0216200e89b60da`
- Spoke-B bridge: `0xf25186b5081ff5ce73482ad761db0eb0d25abfbf`
- Identity registry: `0x8cdaf0cd259887258bc13a92c0a6da92698644c0`

Results:

```text
IdentityRegistry.canTransact(0x627306...) = true
IdentityRegistry.canTransact(0xf17f...) = true

tCeBM.balanceOf(0x627306...) = 0
tCeBM.balanceOf(0xf17f...) = 0

tCeBM.allowance(0x627306..., SpokeBridge) = 0
tCeBM.allowance(0xf17f..., SpokeBridge) = 0

SpokeBridge.hasRole(GOVERNANCE_ROLE, 0x627306...) = true
SpokeBridge.hasRole(GOVERNANCE_ROLE, 0xf17f...) = false
```

Interpretation:

`SpokeBridge.lock(token, amount, txId)` requires:

- caller passes `IdentityRegistry.canTransact(msg.sender)`, and
- `IERC20(token).safeTransferFrom(msg.sender, bridge, amount)` succeeds.

The signer is verified, but has no Spoke-B tCeBM balance or allowance, so the lock transaction reverts. That explains why CB-B never reaches `ACTIVE` and the wizard gets stuck/fails at the commit gate.

## Configuration Smells Found

CB-B `.env` currently contains:

```text
CB_PRIVATE_KEY=c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3
SIGNER_PRIVATE_KEY=c87509a1c067bbde78beb793e6fa76530b6382a4c0241e5e4a9ec0a0f44dc0d3
LOCAL_CB_HUB_SIGNER=0xf17f52151EbEF6C7334FAD080c5704D77216b732
HUB_MINT_RECIPIENT=0xf17f52151EbEF6C7334FAD080c5704D77216b732
```

`CB_PRIVATE_KEY` is CB-A's key, which likely explains why CB-B `/auth/me` returns CB-A's wallet.

CB-B payment-orchestrator intentionally uses `SIGNER_PRIVATE_KEY=0xc875...` per `scenario-b/backend/docker-compose-backend.central-bank-b.yaml`; the compose comment says this signer has `GOVERNANCE_ROLE` on SpokeBridge-B. That is true, but it has no token balance/allowance for `lock`.

CB-A payment-orchestrator startup log says:

```text
spoke_configured=false
spoke bridge not configured — spoke lock/release will be skipped
```

CB-B startup log says:

```text
spoke_configured=true
spoke bridge configured: rpc=http://host.docker.internal:8745 contract=0xf25186b5081ff5ce73482ad761db0eb0d25abfbf
```

So CB-A lock-mint is Hub-only in the current runtime, while CB-B exercises the real SpokeBridge lock path and fails. This asymmetry is probably not intended for a clean wizard E2E.

Also note:

- CB-A local `NATIVE_ASSET_SYMBOL=tCeBM_BRL`
- CB-B local `NATIVE_ASSET_SYMBOL=0xf12b5dd4ead5f743c6baa640b0216200e89b60da`

That means CB-A persisted a symbol in `native_asset`, while CB-B persisted an address. The executor only attempts real spoke lock when the spoke bridge is configured; CB-B is the side hitting the real lock.

## Suggested Next Steps

1. Fix CB-B identity config:
   - Set CB-B `CB_PRIVATE_KEY` to CB-B's actual key if `/auth/me` wallet is expected to be the CB-B wallet.
   - Confirm `/api/v1/auth/me` for CB-B returns wallet `0xf17f52151EbEF6C7334FAD080c5704D77216b732` after restart, if that is the intended identity.

2. Decide desired bridge model for the wizard:
   - If both sides should do real spoke locks, configure CB-A and CB-B symmetrically and ensure the lock caller has token balance and allowance on each spoke.
   - If central-bank liquidity bootstrap should be Hub-only, make CB-B skip spoke lock too or use a separate sovereign-mint flow instead of `SpokeBridge.lock`.

3. For the current CB-B real spoke-lock path, fund and approve the lock caller before retrying:
   - Caller: `0x627306090abaB3A6e1400e9345bC60c78a8BEf57`
   - Token: `0xf12b5dd4ead5f743c6baa640b0216200e89b60da`
   - SpokeBridge: `0xf25186b5081ff5ce73482ad761db0eb0d25abfbf`
   - Required: `balanceOf(caller) >= amount` and `allowance(caller, SpokeBridge) >= amount`

4. Clear or cancel stale pending commit state before rerunning the wizard:
   - CB-A still has pending commit `55b6e1ab-250b-4eb6-8f85-b0baef8d872d` for amount `1000`.
   - Pool status remains `PENDING_COUNTERPART` with zero reserves.

5. Retest endpoint sequence after config/state cleanup:
   - CB-A lock-mint -> ACTIVE -> commit.
   - CB-B lock-mint -> ACTIVE -> commit.
   - Poll commits until `EXECUTED`.
   - Verify pool status becomes `ACTIVE`.
   - Verify final LP positions are present for both CBs.

## Useful Commands

Inspect bridge positions:

```bash
docker exec cbweb3-postgres psql -U default -d cbweb3_central_bank_a \
  -c "select position_id, owner_bank_id, spoke_network, native_asset, mirrored_asset, mirrored_amount, bridge_state, relayer_retries, created_at, updated_at from bridged_asset_positions order by created_at desc limit 10;"

docker exec cbweb3-postgres psql -U default -d cbweb3_central_bank_b \
  -c "select position_id, owner_bank_id, spoke_network, native_asset, mirrored_asset, mirrored_amount, bridge_state, relayer_retries, created_at, updated_at from bridged_asset_positions order by created_at desc limit 10;"
```

Inspect commits:

```bash
docker exec cbweb3-postgres psql -U default -d cbweb3_central_bank_a \
  -c "select commit_id, pool_pair, provider_id, side, amount, status, counterpart_commit_id, created_at, expires_at from pool_commits order by created_at desc limit 10;"

docker exec cbweb3-postgres psql -U default -d cbweb3_central_bank_b \
  -c "select commit_id, pool_pair, provider_id, side, amount, status, counterpart_commit_id, created_at, expires_at from pool_commits order by created_at desc limit 10;"
```

Check CB-B spoke lock prerequisites:

```bash
RPC=http://localhost:8745
IR=0x8cdaf0cd259887258bc13a92c0a6da92698644c0
TOKEN=0xf12b5dd4ead5f743c6baa640b0216200e89b60da
BRIDGE=0xf25186b5081ff5ce73482ad761db0eb0d25abfbf
SIGNER=0x627306090abaB3A6e1400e9345bC60c78a8BEf57
CBB=0xf17f52151EbEF6C7334FAD080c5704D77216b732
GOV=$(cast keccak 'GOVERNANCE_ROLE')

cast call "$IR" 'canTransact(address)(bool)' "$SIGNER" --rpc-url "$RPC"
cast call "$TOKEN" 'balanceOf(address)(uint256)' "$SIGNER" --rpc-url "$RPC"
cast call "$TOKEN" 'allowance(address,address)(uint256)' "$SIGNER" "$BRIDGE" --rpc-url "$RPC"
cast call "$BRIDGE" 'hasRole(bytes32,address)(bool)' "$GOV" "$SIGNER" --rpc-url "$RPC"
```
