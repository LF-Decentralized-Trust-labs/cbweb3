# Research — TK-B5 (relay generalizado)

Fase 0. Decisões. Formato: Decisão / Rationale / Alternativas.

## R1 — Extração de lógica pura para testabilidade

- **Decisão**: extrair `spoke-registry`, `relay-store`, `circuit-breaker` (checagem `isPaused`) e a
  **validação de payload** em módulos **puros** (sem dependência do Cacti connector nem de um Besu
  real), com o `provider`/`fetch` injetáveis; `index.ts` fica fino (fiação: cria conectores/watchers
  a partir do registry).
- **Rationale**: os conectores Cacti/Besu exigem uma rede real — não testáveis unitariamente. A
  lógica generalizada (N spokes, roteamento por `spoke_out`, breaker, persistência) é o que a spec
  exige e é 100% testável isolando as dependências. Cumpre o Princípio V sem subir Besu.
- **Alternativas**: testes E2E com Besu — caros/lentos e fora do escopo do TK-B5 (ficam para
  TK-B10); rejeitados como via primária.

## R2 — Framework de teste do relay

- **Decisão**: **`node:test`** (nativo do Node 20) + `ts-node` (já é devDependency) para os testes
  da lógica pura; script `test` no `package.json` (ex.: `node --import ts-node/register --test`).
- **Rationale**: zero dependência nova; o relay já traz `ts-node`. Runner nativo é suficiente para
  asserções unitárias.
- **Alternativas**: Jest/Vitest — dependência nova pesada, desnecessária para o escopo; rejeitados.

## R3 — Persistência do RelayStore

- **Decisão**: **arquivo JSON** (`fs`) num caminho configurável (volume do relay), com
  `load()` no boot e `save()` a cada registro; escrita atômica (write-tmp + rename). Substitui o
  `cacti-relay-store.json` legado/plano.
- **Rationale**: simples, sem servidor de estado; sobrevive a reinícios (SC-003); casa com a feature
  `031-relay-spoke-registry` descrita no roadmap. Sem nova dependência.
- **Alternativas**: SQLite/Redis — dependência e operação a mais sem ganho nesta fase; rejeitados.

## R4 — Checagem de `isPaused()` (circuit breaker)

- **Decisão**: consultar `isPaused()` no `AutomatedMarketMaker` do par via **ethers v6** (já
  presente) com o endereço do AMM resolvido pelo par/registry; **falha segura** — se a consulta
  lançar/expirar, **não encaminhar** (tratar como pausado).
- **Rationale**: Princípio III exige validar o breaker antes do swap; a falha segura evita
  encaminhar durante incerteza. Sem alterar contratos (só leitura).
- **Alternativas**: assumir "ativo" em erro — inseguro, viola a intenção do breaker; rejeitado.

## R5 — Registro em runtime e idempotência

- **Decisão**: `POST /api/v1/spokes` valida o payload (id, RPC/WS, gateway), faz **upsert** por id
  (idempotente), persiste no store e **hidrata em runtime** (cria conector/watcher + rota) sem
  restart; payload inválido → 400 sem alterar o store.
- **Rationale**: atende US2/US3/SC-002 e o modelo declarativo N-spokes; upsert evita duplicar
  conector no re-registro.
- **Alternativas**: registro só via arquivo + restart (MVP do roadmap) — pior UX no relay neutro
  compartilhado; o roadmap decidiu pelo runtime.

## R6 — `RelayRegistrar` (Go): factory por URI

- **Decisão**: `New(uri)` → `local` (in-memory idempotente) | `relay://<host>` (stub de produção que
  faz `POST /api/v1/spokes` via `net/http`) | outro → erro de configuração. Erros tipados; sem
  segredos.
- **Rationale**: espelha `keyprovider`/`certsource` (TK-B2/B3) — consistência de padrão plugável;
  o motor (TK-B6) escolhe a impl por URI.
- **Alternativas**: acoplar o registro direto no motor — quebra o padrão de fronteira plugável;
  rejeitado.

## R7 — Auth por CB (fora de escopo)

- **Decisão**: **não** implementar auth-por-CB (§14.D); manter o **segredo compartilhado**
  (`X-Relay-Auth`) atual como fallback local/dev; documentar a recomendação para fase posterior.
- **Rationale**: sensível a compliance; a Constituição exige aprovação do **project-lead** para
  mudar controles de segurança. Preservar o atual não é enfraquecimento.
- **Alternativas**: implementar agora — bloqueado por falta de sign-off; rejeitado nesta fase.

**Saída**: nenhuma `NEEDS CLARIFICATION` remanescente.
