# Research — TK-B6 (motor + found-hub + hub bundle + apply)

Fase 0. Decisões. Formato: Decisão / Rationale / Alternativas.

## R1 — Motor de steps (reimplementação do padrão de referência)

- **Decisão**: reimplementar `Step{Name,Check,Run,Deps}` + `Orchestrator` (execução topológica,
  `Check`→skip / `Run`→executa+persiste) + estado YAML por step + `flock`, espelhando
  `scenario-a/toolkit/engine/orchestrator`.
- **Rationale**: padrão validado; idempotência e retomada são requisitos (FR-001..003).
- **Alternativas**: importar o do Cenário A — proibido (Princípio I); framework de workflow externo —
  dependência desnecessária.

## R2 — Deploy dos contratos do hub = um `forge script`

- **Decisão**: o step `deploy-hub-contracts` roda **um** `forge script
  script/CBWeb3Hub.s.sol:DeployCBWeb3Hub --rpc-url <hub> --broadcast` (com `DEPLOYER_PRIVATE_KEY` e
  `CENTRAL_BANK_ADDRESS` no env), e extrai os endereços do **broadcast JSON**
  (`contracts/broadcast/CBWeb3Hub.s.sol/<chainId>/run-latest.json`). A ordem
  IdentityRegistry→tCeBM_BRL→tCeBM_EUR→FXAgreement→PairRegistry→CurrencyRegistry→ManualOracle é
  **interna ao script Solidity** (verificado em `CBWeb3Hub.s.sol`).
- **Rationale**: é exatamente o que o Makefile `contracts.deploy-hub` faz (spec executável de
  referência); um único script garante a ordem atômica. O `Check()` do step verifica se os endereços
  já existem/respondem (idempotência).
- **Alternativas**: N steps separados por contrato — desnecessário e divergente da fonte; rejeitado.
  Gate de **build**: `forge build` (ou `contracts.setup`+`build`) antes, pois `contracts/out/` pode
  estar vazio.

## R3 — Executor injetável (efeitos externos)

- **Decisão**: `engine/exec.CommandRunner` com impl. **real** (`os/exec`: docker compose, `forge`,
  `kcadm`/`curl`) e **fake** (grava os comandos) para testes; `--dry-run` usa um runner que **não
  executa** (só registra o plano).
- **Rationale**: torna o motor e os steps testáveis sem Docker/Foundry e garante o contrato do
  dry-run (FR-005/FR-013).
- **Alternativas**: chamar `os/exec` direto nos steps — não testável; rejeitado.

## R4 — Keycloak: provisionamento + write-back (confirmado necessário)

- **Decisão**: `provision-keycloak-hub` sobe o Keycloak (template TK-B4), aguarda `/realms/master`,
  cria realm/client do **operador do hub** e faz **write-back** dos client secrets JWT nos `.env` dos
  backends do hub; idempotente (não regrava se já presente). Reproduz o `deploy/local/keycloak/init.sh`.
- **Rationale**: o hub roda o **portal de governança** e o **NOC** (OIDC) e o **api-gateway** aplica o
  gate de compliance (Constituição IV) — decisão do usuário (2026-07-10): **manter**. Sem write-back,
  os serviços do hub não validam tokens nem agem como service account.
- **Alternativas**: hub mínimo sem Keycloak — rejeitado pelo usuário; contornar o gate — proibido (IV).

## R5 — Hub bundle (RPC-only, sem segredos)

- **Decisão**: `engine/bundle` emite `bundles/hub.bundle.yaml` versionado com: endereços dos 7
  contratos do hub, `chainId`, RPC/WS do hub e config pública de consumo. **Sem segredos.**
  `load`+`validate` fazem round-trip. Não embute genesis/enode (RPC-only; diferente do spoke bundle).
- **Rationale**: hand-off para `found-spoke` (TK-B7) sem hardcode; §11 do roadmap.
- **Alternativas**: incluir chaves/PII — viola II/FR-012; rejeitado.

## R6 — Suíte E2E com skip-com-aviso

- **Decisão**: `tests/e2e` sob **build tag `e2e`**; um pré-check detecta `forge`, `docker` e a imagem
  `hyperledger/besu:25.8.0` — ausentes ⇒ `t.Skip` com aviso explícito. Quando presente, roda
  `apply found-hub` real e valida os endereços do bundle contra o RPC (código on-chain != vazio).
- **Rationale**: decisão do usuário (E2E completo) sem travar o `go test` padrão nem gerar falso
  verde (FR-015/SC-009).
- **Alternativas**: E2E sempre-on — trava CI sem o ambiente; só-unit — não atende a decisão E2E.

## R7 — Estado e lock

- **Decisão**: `<dataDir>/.provisioning-state.yaml` (status por step) + `flock`
  `<dataDir>/.provisioning.lock`; lock órfão detectado por PID/idade com política clara (erro
  acionável, não trava eterno).
- **Rationale**: FR-002/FR-003; espelha o toolkit de referência.

**Saída**: nenhuma `NEEDS CLARIFICATION` remanescente.
