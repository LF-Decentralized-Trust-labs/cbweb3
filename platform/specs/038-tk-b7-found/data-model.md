# Data Model — TK-B7 (found-spoke + spoke bundle)

Fase 1. Entidades, regras e estados. Reusa o motor/estado/report do TK-B6.

## SpokeConfig (parametriza o found-spoke)

- **Campos**: `Runner`, `ContractsDir`, `TemplatesDir`, `OutDir`, `SpokeID`, `SpokeChainID`,
  `SpokeRPC`, `SpokeWS`, `CBAddress` (CENTRAL_BANK), `GenesisDir`, `HubBundlePath`, `HubRPC` (do
  bundle), `SpokeEnvFile`, `KeycloakEnv []string`, `GatewayURL` (para o relay).
- **Seams injetáveis**: `WaitRPC`, `WaitKeycloak`, `ReadClientSecret`, `EnodeReader`, `Registrar`
  (RelayRegistrar).
- **Regras**: `HubBundlePath` obrigatório; `SpokeChainID` ≠ hub chainId (validação de colisão fica no
  manifesto/TK-B1).

## Step set do `found-spoke` (em ordem de deps)

1. `consume-hub-bundle` — `bundle.LoadHub(HubBundlePath)`; falha clara se inválido. (base)
2. `register-cb` — contra `HubRPC`, **duas ações idempotentes**: `registerParticipant(CENTRAL_BANK)`
   (`RegisterParticipants.s.sol`; Check `isParticipant`) + `grantLiquidityProvider(CB)` automático
   (Check `isLiquidityProvider`; `onlyRole(DEFAULT_ADMIN_ROLE)` — pendência não-fatal se sem
   permissão fora de local). (dep: consume-hub-bundle)
3. `build-contracts` — `forge build` (gate: `out/`).
4. `gen-genesis-spoke` — genesis QBFT da chain do spoke (`genGenesisStep`). (dep: build? não; base)
5. `start-besu-spoke` — sobe o nó do spoke (template `entity-besu`/hub-like), CB validador; gate RPC;
   captura enode. (dep: gen-genesis-spoke)
6. `deploy-spoke-contracts` — `forge script CBWeb3Spoke.s.sol` contra `SpokeRPC`; grant GOVERNANCE ao
   CB. `Check`: broadcast presente. (dep: build-contracts, start-besu-spoke)
7. `wire-hub-addresses` — endereços do hub (bundle) → `SpokeEnvFile` (idempotente). (dep:
   consume-hub-bundle)
8. `provision-keycloak-spoke` — realm/client + write-back. (dep: deploy-spoke-contracts)
9. `render-spoke-env` — endereços do spoke → env. (dep: deploy-spoke-contracts)
10. `start-spoke-infra` / `backend` / `frontend`. (deps encadeadas)
11. `register-relay-spoke` — `Registrar.Register(spokeID, WS, gateway)`; idempotente. (dep:
    start-besu-spoke)
12. `add-noc-agent` — **Soft**; sobe noc-agent do spoke. (dep: start-besu-spoke)
13. `emit-spoke-bundle` — genesis + enode + chainId + endereços → `bundles/spoke-<id>.bundle.yaml`.
    (dep: deploy-spoke-contracts, start-besu-spoke)

## Step (extensão — soft)

- **Campo novo**: `Soft bool`. Um step `Soft` que falha ⇒ `soft-failed` no report; **não** interrompe
  o motor. Steps não-soft mantêm o comportamento do TK-B6 (interrompe + persiste).

## Enode (captura)

- **`EnodeReader func(ctx, rpcURL) (string, error)`** — real: JSON-RPC `admin_nodeInfo` → campo
  `enode`; fake nos testes. O enode do CB entra no spoke bundle.

## SpokeBundle

- **Campos**: `Version`, `SpokeID`, `ChainID`, `Enode`, `SpokeRPC`, `SpokeWS`, `Genesis` (conteúdo do
  genesis.json), `Contracts {identityRegistry, tCeBM, spokeBridge, fCeBM}`.
- **Regras**: versionado; **sem segredos** (rejeita bloco `PRIVATE KEY`); `EmitSpoke`→`LoadSpoke`
  round-trip; genesis/enode são públicos; consumido pelo `join` (TK-B8).

## Report (reuso + soft)

- `StepResult.Status` ganha o valor `soft-failed` (além de done/skipped/failed/planned).

## Transições (found-spoke)

```
consume-hub-bundle → register-cb ┐
build-contracts ─────────────────┤
gen-genesis-spoke → start-besu-spoke (captura enode) ┐
                                                      ├→ deploy-spoke-contracts → provision-keycloak → render-env
wire-hub-addresses ──────────────────────────────────┘        → infra/backend/frontend
start-besu-spoke → register-relay-spoke ; add-noc-agent (soft)
deploy-spoke-contracts + start-besu-spoke → emit-spoke-bundle
cada step: Check? skip : (dryRun? planned : Run→done|failed|soft-failed)
```
