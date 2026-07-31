# Phase 0 — Research: TK-B8 join (full node não-validador)

Todos os "NEEDS CLARIFICATION" foram resolvidos no `/speckit.clarify` (fluxo canônico sem relay/noc;
wire só endereços de spoke). As decisões técnicas abaixo consolidam o reuso e as extensões.

## D1 — Reuso do motor TK-B6/B7

- **Decisão**: reusar `orchestrator` (Step{Name,Deps,Soft,Check,Run}, topoSort, estado YAML, `flock`,
  dry-run, Report), `exec.CommandRunner` (Real/Dry/Fake), `engine/addrs` (AppendAddr upsert idempotente,
  HasAddrKey), `engine/bundle` (`LoadSpoke`), `engine/pki` (`GenerateBankCSR`) e os templates (TK-B4).
- **Rationale**: `join` é o terceiro modo sobre o mesmo motor; nada novo de infraestrutura.
- **Alternativas**: reimplementar gate/estado — rejeitado (duplicação, contra Princípio I de coesão).

## D2 — `write-genesis` (copiar do bundle) vs `gen-genesis` (gerar)

- **Decisão**: `write-genesis` **escreve o genesis do spoke a partir do bundle** (`SpokeBundle.Genesis`)
  em `<GenesisDir>/genesis.json`, com **guard não-destrutivo**: se o arquivo já existe, comparar o
  `sha256` do conteúdo do bundle com o do arquivo; iguais → skip (idempotente); divergentes → **erro
  claro** (o banco não pode rodar um genesis diferente do do spoke).
- **Rationale**: o banco DEVE usar exatamente o genesis do CB (mesma chainId, validadores QBFT = só o
  CB). Regenerar (via `besu operator`) produziria material divergente. FR-002/SC-002.
- **Alternativas**: reusar `genGenesisStep` (gera) — rejeitado (join não gera; consome o do bundle).

## D3 — `wait-sync` (gate de sincronização)

- **Decisão**: novo gate `wait-sync` que faz `eth_syncing` via JSON-RPC contra o RPC do nó do banco;
  concluir quando `eth_syncing == false` **e** `eth_blockNumber` > 0 (o nó pegou a cadeia). Injetável
  via seam `EthSyncing func(ctx, rpcURL) (syncing bool, block uint64, err error)` (default = POST
  `eth_syncing` + `eth_blockNumber`); timeout com erro claro (nunca verde falso).
- **Rationale**: FR-004/SC-003 — o banco só é útil sincronizado. Análogo ao `waitRPC` do TK-B6, mas
  verifica *sync*, não só *RPC no ar*. Testável sem Besu (fake seam).
- **Alternativas**: só `waitRPC` — rejeitado (RPC no ar ≠ sincronizado; falso verde).

## D4 — Não-validador (CB é validador único)

- **Decisão**: `start-besu-join` sobe o nó do banco como full node **não-validador** (peer do enode do
  CB, do bundle), **sem** `vote-qbft`. `node.validator: true` no manifesto → **warning** (já
  implementado em `manifest/validate.go`), o banco ainda entra como não-validador.
- **Rationale**: roadmap §6/§14 (modelo do Cenário A; genesis `count: 1`). A promoção a validador é
  capacidade diferida (ADR-002), fora do `join`. FR-003/FR-011/SC-006.
- **Alternativas**: 2 validadores (como o `startBesu.sh` do `deploy/local`) — rejeitado (diverge do
  modelo decidido; o toolkit segue o Cenário A).

## D5 — `gen-csr` (cauda diferida de PKI)

- **Decisão**: `gen-csr` reusa `pki.GenerateBankCSR(bankCode, institution, <dataDir>/pki)` — key
  `0600`, CSR `OU=ROLE_COMMERCIAL_BANK`, CN=`bankId`. Acréscimos: (a) **idempotência** (Check: pula se
  `{bank}.key` **e** `{bank}.csr` já existem); (b) **pré-criar `<dataDir>/pki/`** como usuário do host
  (MkdirAll `0700`) antes de qualquer mount do compose — evita o dir root-owned que quebra o gen-csr
  (bug do Cenário A). O toolkit **nunca** assina o CSR, **nunca** gera CA, **nunca** transmite a chave.
- **Rationale**: FR-008/FR-009/FR-010/SC-005; roadmap §15.6. A assinatura (compliance do CB), a emissão
  gated por KYC e o registro on-chain são runtime — mantê-los fora evita a colisão de wallet (registro
  keyed na wallet do toolkit em vez da wallet KMS do banco).
- **Alternativas**: toolkit assina/registra — rejeitado explicitamente (colisão de wallet; Cenário A).

## D6 — `wire-addresses` (só endereços de spoke)

- **Decisão**: `wire-addresses` escreve os endereços de spoke do bundle
  (`identityRegistry`/`tCeBM`/`spokeBridge`/`fCeBM`) no `.env` do banco via `addrs.AppendAddr`
  (upsert idempotente). **Não** consome hub addresses (não estão no bundle do TK-B7 — clarificação).
- **Rationale**: FR-005/SC-004; decisão de escopo (não tocar o `emit-spoke-bundle` do TK-B7).
- **Alternativas**: estender o bundle p/ hub addresses — rejeitado nesta fase (follow-up separado).

## D7 — Sem relay/noc no `join` (fluxo canônico)

- **Decisão**: o `join` **não** tem `register-relay-bank` nem `add-noc-agent`. A cadeia do spoke já é
  registrada/observada no relay desde o `found-spoke`; um full node de banco é apenas mais um peer/RPC
  na mesma cadeia.
- **Rationale**: roadmap §6 (fluxo canônico) + clarificação. Evita registro por-banco redundante.
- **Alternativas**: registrar cada banco no relay — rejeitado (redundante; fora do fluxo canônico).

## D8 — Dispatch `join` no `apply` + CLI

- **Decisão**: `apply.go` troca o `case "join"` (hoje "not supported yet (TK-B8)") por `applyJoin`,
  que valida o spoke bundle cedo (`bundle.LoadSpoke`), monta `JoinConfig` do manifesto + flags e roda
  `JoinSteps`. `main.go` atualiza doc/usage (join suportado). Reusa flags existentes
  (`--spoke-rpc` = RPC do nó do banco no gate `wait-sync`; `--data-dir`, `--out-dir`, `--repo-root`).
- **Rationale**: consistente com `applyFoundSpoke`/`applyFoundHub`; erro cedo em bundle inválido
  (FR-001/SC-001) antes de qualquer efeito.
- **Alternativas**: novo subcomando — rejeitado (o dispatch por `spec.mode` já é o padrão).
