# Phase 0 — Research: TK-B9 par soberano + liquidez cooperativa + seed-oracle

NEEDS CLARIFICATION resolvidos no `/speckit.clarify` (Q1 = cauda soft do found-spoke; Q2 = soberania
estrita, cada run só o ato do CB corrente). As decisões técnicas abaixo consolidam reuso e deviations.

## D1 — Placement: cauda soft do found-spoke (Q1)

- **Decisão**: `open-sovereign-pair`, `commit-liquidity`, `seed-oracle` são anexados a `FoundSpokeSteps`
  quando `SpokeConfig.Pair != nil`, com `Deps: ["add-noc-agent"]`; `emit-spoke-bundle` passa a depender
  dos três. Todos `Soft: true`.
- **Rationale**: roadmap §404-407; `spec.pair` já é opcional em found-spoke. Sem par → não anexa.
- **Alternativas**: modo standalone `open-pair` — rejeitado (Q1; diverge do roadmap, exige novo dispatch/
  manifesto).

## D2 — Soberania estrita: atos por ator, sem chave da contraparte (Q2)

- **Decisão**: cada `apply` executa só o ato soberano do CB do manifesto atual. `open-sovereign-pair`
  ramifica pelo papel (comparando o CB corrente a `pair.proposerCB`/`pair.confirmerCB`):
  - **Proponente (CB-A)**: scaffolding (deploy/dedup W-tokens, AMM, LCR; `setCentralBankOf`; grant ao
    relayer) com a chave de **admin do hub**, depois `proposePair` com a chave de **CB-A**.
  - **Confirmador (CB-B)**: `confirmPair` com a chave de **CB-B** (só quando `getPair == PROPOSED`).
- **Rationale**: Q2 (nenhum run detém a chave da contraparte). Scaffolding é ato de admin do hub
  (roadmap §448), disponível localmente sem a chave de CB-B.
- **Alternativas**: um run detém ambas as chaves — rejeitado (Q2).

## D3 — Não reusar `SeedNewSovereignPair.s.sol`; atos discretos via executor

- **Decisão**: **não** invocar o script monolítico (exige `CB_A_HUB_PRIVATE_KEY` **e**
  `CB_B_HUB_PRIVATE_KEY` + faz propose+confirm juntos). Em vez disso, orquestrar via o executor:
  - Scaffolding: `forge create` por contrato (W-token por moeda com dedup; `AutomatedMarketMaker(tokenA,
    tokenB, hubIdentityRegistry)`; `LiquidityCommitRegistry(hubIdentityRegistry)` reutilizando se já
    existir) + `cast send` para `setCentralBankOf` e `grantRole(CENTRAL_BANK_ROLE, relayer)`.
  - `cast send` para `proposePair` / `confirmPair` / `registerCommit` / `setRate`.
- **Rationale**: honra a soberania estrita (Q2) e "não alterar contratos/scripts". `cast` já é usado no
  `register-cb` (grantLiquidityProvider). Endereços vêm do stdout do `forge create`/broadcast.
- **Alternativas**: novo `.s.sol` dividido — rejeitado (altera contratos). Reusar o monolítico —
  rejeitado (Q2).

## D4 — Idempotência on-chain via `getPair` status

- **Decisão**: `Check()` de `open-sovereign-pair` faz `cast call <pairRegistry> "getPair(string)"
  <pairId>` (via runner) e interpreta o status: `ACTIVE` → skip; `PROPOSED` + eu sou o proponente →
  skip (já propus, aguardando); `PROPOSED` + eu sou o confirmador → run (`confirmPair`); inexistente
  (revert) + eu sou o proponente → run (`proposePair`); inexistente + eu sou o confirmador → `pending`.
  `pairId` determinístico: `W-<symbolA>-<symbolB>`.
- **Rationale**: FR-006; o estado vive on-chain, então re-`apply` converge. Via `cast` no executor →
  testável com FakeRunner.
- **Alternativas**: `eth_call` cru + ABI-decode do struct — rejeitado (mais complexo; `cast call` já
  decodifica e passa pelo mesmo seam de executor dos outros atos).

## D5 — commit-liquidity: só a moeda do CB corrente

- **Decisão**: `commit-liquidity` faz `cast send <LCR> "registerCommit(string,uint8,uint256,address)"
  <poolPair> <side> <amount> <wToken>` **apenas** para o lado da moeda do CB corrente. O casamento
  (`CommitMatched`) é do **relay** ao observar ambos os lados. `Check`: pula se já há commit pendente/
  casado do lado do CB (lê `getPendingCommit(poolPair, side)`).
- **Rationale**: FR-007/FR-008; design cooperativo (cada CB só a sua moeda). Reforça o Princípio II.
- **Alternativas**: o toolkit casar os dois lados — rejeitado (é papel do relay; violaria soberania).

## D6 — seed-oracle: local-only, idempotente

- **Decisão**: `seed-oracle` faz `cast send <manualOracle> "setRate(address,address,uint256)" <tokenA>
  <tokenB> <rate>` **apenas** em `environment: local`; fora de `local` → `Check` retorna skip. Taxa vem
  do manifesto (campo do `spec.pair`, ou default local). Idempotente (re-setRate com a mesma taxa é
  inócuo; opcionalmente `Check` lê `getRate`).
- **Rationale**: FR-009; roadmap §766-771 (seed one-shot, detalhe só de local).
- **Alternativas**: feeder contínuo (jitter) — fora de escopo (ajuste futuro, só local).

## D7 — Soft steps + emit-bundle não bloqueado

- **Decisão**: os três steps são `Soft: true` (reusa TK-B7). `emit-spoke-bundle` ganha Deps neles para
  ordem, mas como soft-failed não interrompe o motor (TK-B6), o bundle é emitido mesmo se a cauda
  soberana falhar/ficar pending.
- **Rationale**: FR-002/SC-006; roadmap §404-407 (cauda soft antes do bundle).
- **Alternativas**: steps hard — rejeitado (bloquearia o found-spoke por um par bilateral pendente).

## D8 — Chaves e parâmetros via apply/flags

- **Decisão**: `applyFoundSpoke` popula `SpokeConfig.Pair` (proposerCB/confirmerCB/symbolA/symbolB do
  `spec.pair`) e resolve: chave do CB corrente (KeyProvider/flag), chave de admin do hub (local),
  endereços de `pairRegistry`/`manualOracle`/`identityRegistry` (do hub bundle já consumido) e o
  endereço do relayer. Em `local`, as chaves são derivadas/local; fora de `local`, ausência → `pending`.
- **Rationale**: reusa o hub bundle (endereços) e o padrão de config injetável do found-spoke.
- **Alternativas**: novo manifesto standalone — rejeitado (Q1).
