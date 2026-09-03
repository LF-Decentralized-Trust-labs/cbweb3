# Data Model — TK-B6 (motor + found-hub + hub bundle + apply)

Fase 1. Entidades, regras e estados.

## Step

- **Campos**: `Name` (único), `Deps []string` (pré-requisitos), `Check(ctx) (bool, error)`
  (satisfeito?), `Run(ctx, deps) error` (efeito idempotente).
- **Regras**: `Check` true ⇒ step pulado; `Run` só após todos os `Deps` concluídos; `Run` persiste o
  status ao concluir.

## Orchestrator (motor)

- **Estado**: lista de steps + `State` + `Lock` + `Runner` (executor) + flag `dryRun`.
- **Operações**: `Run(ctx)` — ordena topologicamente; para cada step: se `Check` ⇒ `skipped`; senão
  (dry-run) `planned` / (real) `Run` → `done`|`failed`; para no primeiro `failed`.
- **Regras**: ordem topológica estável; falha interrompe e persiste; retomada pula `done`.

## State (`.provisioning-state.yaml`)

- **Campos**: `map[stepName] -> {status: done|failed|skipped, ts, detail}`.
- **Regras**: durável no `dataDir`; lido no início (retomar); escrito por step.

## Lock (`.provisioning.lock`)

- **Campos**: `flock` + metadados (pid, ts).
- **Regras**: exclusivo; segunda execução concorrente recusada; lock órfão (pid morto/idade) tratado
  por política clara (erro acionável).

## Report

- **Campos**: `mode`, `steps[] {name, status: done|skipped|failed|planned, detail}`, `bundlePath?`.
- **Regras**: emitido **sempre** em stdout (json|yaml); report **parcial** em interrupção (sinal).

## Executor (`CommandRunner`)

- **Interface**: `Run(ctx, cmd, args..., opts) (output, error)`.
- **Impls**: real (`os/exec`), fake (grava invocações; para testes), dry (no-op que registra plano).
- **Regras**: todo efeito externo (docker compose, `forge`, kcadm/curl) passa por aqui; dry-run nunca
  aciona a impl. real.

## Steps do `found-hub` (inventário, em ordem de deps)

1. `build-contracts` — `forge build` (gate: artefatos em `contracts/out/`).
2. `start-besu-hub` — sobe o nó validador do hub (template TK-B4 `hub`). Gate: RPC no ar.
3. `deploy-hub-contracts` — `forge script CBWeb3Hub.s.sol:DeployCBWeb3Hub` (dep: start-besu-hub).
   `Check`: endereços já respondem no RPC.
4. `provision-keycloak-hub` — sobe Keycloak; cria realm/client do operador; **write-back** secrets.
   Gate: `/realms/master`. Idempotente.
5. `render-hub-env` — grava endereços/config nos `.env` do hub (dep: deploy + keycloak).
6. `start-hub-infra` / `start-hub-backend` / `start-hub-frontend` — sobe infra/backend/frontend.
7. `start-relay` — sobe o relay **neutro** (TK-B5).
8. `start-noc` — sobe `noc-db`/`noc-backend`/`noc-portal` (neutro).
9. `emit-hub-bundle` — emite `bundles/hub.bundle.yaml` (dep: deploy-hub-contracts).

## HubBundle (`bundles/hub.bundle.yaml`)

- **Campos**: `version`, `chainId`, `hubRpc`, `hubWs`, `contracts {identityRegistry, tCeBM_BRL,
  tCeBM_EUR, fxAgreement, pairRegistry, currencyRegistry, manualOracle}`.
- **Regras**: versionado; **sem segredos** (só dados públicos); `load`+`validate` round-trip; usado
  pelo `found-spoke` (TK-B7).

## Transições (found-hub)

```
build → start-besu-hub → deploy-hub-contracts → provision-keycloak → render-env
      → infra/backend/frontend → start-relay → start-noc → emit-hub-bundle
cada step: Check? skip : (dryRun? planned : Run→done|failed)
```
