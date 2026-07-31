# Research — TK-B7 (found-spoke + spoke bundle)

Fase 0. Decisões. Formato: Decisão / Rationale / Alternativas.

## R1 — Reuso máximo do TK-B6

- **Decisão**: reusar `engine/orchestrator` (motor/estado/lock/report), `engine/exec` (runner),
  `engine/addrs` (broadcast Foundry), `engine/bundle` e as interfaces `KeyProvider`/`CertSource`/
  `RelayRegistrar`. O `found-spoke` é um novo **step set** + um novo **bundle** + o **dispatch** no
  `apply`.
- **Rationale**: o motor é modo-agnóstico; o TK-B7 só acrescenta o inventário de steps do spoke.
- **Alternativas**: motor separado por modo — duplicação desnecessária; rejeitado.

## R2 — Deploy dos contratos do spoke = um forge script

- **Decisão**: `deploy-spoke-contracts` roda `forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke
  --rpc-url <spoke> --broadcast` (com `CENTRAL_BANK_ADDRESS`), deployando IdentityRegistry → tCeBM
  doméstico → SpokeBridge → fCeBM (ordem interna ao Solidity) + grant `GOVERNANCE_ROLE` ao CB. Os
  endereços saem do broadcast (`contracts/broadcast/CBWeb3Spoke.s.sol/<chainId>/run-latest.json`).
- **Rationale**: é o que o Makefile `contracts.deploy-spoke-*` faz (spec executável de referência);
  reusa `engine/addrs.ParseBroadcastList`.
- **Alternativas**: N steps por contrato — divergente; rejeitado.

## R3 — register-cb contra o hub (duas ações; resolução A1)

- **Decisão**: `register-cb` faz **duas ações idempotentes** contra o RPC do hub: (1)
  `forge script RegisterParticipants.s.sol:RegisterParticipants` → `registerParticipant(CENTRAL_BANK)`
  (Check `isParticipant`); (2) **`grantLiquidityProvider(CB)`** no `IdentityRegistry` do hub, tentado
  **automaticamente** (Check `isLiquidityProvider`) — reproduz `contracts.register-participants-hub` +
  `contracts.grant-liquidity-providers` do Makefile. Verificado: `RegisterParticipants.s.sol` **só**
  registra CENTRAL_BANK; o grant é passo separado (`cast send grantLiquidityProvider`),
  `onlyRole(DEFAULT_ADMIN_ROLE)`.
- **Rationale**: FR-002 exige ambos; o grant é pré-requisito de provedor de liquidez (commit-liquidity
  em TK-B9). Em `local` o toolkit detém a chave do **admin do hub** e completa o grant; fora de
  `local`, sem permissão, o grant vira **pendência não-fatal** (governança do hub autoriza depois).
- **Alternativas**: só registrar CENTRAL_BANK (C) — deixaria o CB sem papel de liquidez; dois steps
  separados (B) — mais verboso; rejeitados. Escolhida a opção **A** (2026-07-11).

## R4 — gen-genesis generalizado (hub e spoke)

- **Decisão**: generalizar o `genGenesisHubStep` do TK-B6 para `genGenesisStep(name, chainID,
  genesisDir, validators, runner, image)`; `found-spoke` usa-o com a **chain do spoke** e o CB como
  validador. Idempotente (genesis já presente → pula).
- **Rationale**: mesma mecânica (`besu operator generate-blockchain-config`); evita duplicar.
- **Alternativas**: novo gerador só p/ spoke — duplicação; rejeitado.

## R5 — Captura do enode (para o spoke bundle)

- **Decisão**: após `start-besu-spoke`, capturar o **enode** do nó do CB via `admin_nodeInfo`
  (JSON-RPC), com a função injetável (`EnodeReader`) — real via `net/http`, fake nos testes. O enode
  entra no spoke bundle (o `join` do TK-B8 usa-o como `--bootnodes`).
- **Rationale**: o spoke bundle precisa do enode para os bancos joinarem (§11); RPC-only não basta.
- **Alternativas**: ler enode do volume/keystore — mais frágil; `admin_nodeInfo` é a fonte canônica.

## R6 — Steps soft (não-fatais)

- **Decisão**: adicionar `Step.Soft bool`; no motor, um step `Soft` que falha vira **`soft-failed`**
  no report e **não interrompe** a execução (os demais steps prosseguem). `add-noc-agent` é `Soft`.
- **Rationale**: o roadmap marca `add-noc-agent` como soft (observabilidade não bloqueia a fundação);
  extensão mínima do motor.
- **Alternativas**: tratar noc-agent fora do motor — perde idempotência/estado; rejeitado.

## R7 — Spoke bundle (genesis + enode, sem segredos)

- **Decisão**: `engine/bundle.SpokeBundle{Version,SpokeID,ChainID,Enode,SpokeRPC,SpokeWS,Genesis
  (conteúdo do genesis.json embutido ou referência), Contracts}`; `EmitSpoke` grava
  `bundles/spoke-<id>.bundle.yaml`; `ValidateSpoke` exige campos e **rejeita** material de chave
  privada. O genesis é público (config de rede), não segredo.
- **Rationale**: o `join` (TK-B8) consome genesis+enode+chainId+endereços; §11 (spoke bundle lê
  genesis/enode dos volumes).
- **Alternativas**: bundle RPC-only (como o hub) — insuficiente para o join; rejeitado.

## R8 — consume-hub-bundle

- **Decisão**: `consume-hub-bundle` usa `bundle.LoadHub` (TK-B6) a partir de um caminho referenciado
  no manifesto (`spec.hubBundleRef`); falha clara se ausente/inválido; disponibiliza os endereços do
  hub para `register-cb` e `wire-hub-addresses`.
- **Rationale**: hand-off do TK-B6; sem hardcode.

## R9 — E2E skip-com-aviso (pressupõe hub fundado)

- **Decisão**: `tests/e2e/found_spoke_e2e_test.go` (build tag `e2e`) requer Docker/forge/besu **e** um
  hub bundle de um hub fundado (env `CBWEB3B_E2E_HUB_BUNDLE`, `..._SPOKE_RPC`, etc.); ausência ⇒
  `t.Skip` com aviso. Quando presente, funda o spoke e valida o spoke bundle.
- **Rationale**: consistente com o TK-B6; o found-spoke depende de um hub real.

**Saída**: nenhuma `NEEDS CLARIFICATION` remanescente.
