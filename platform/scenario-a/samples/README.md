# Samples — Provisionamento de dois spokes (Brasil e Colômbia)

Este diretório contém manifestos `ParticipantDeployment` prontos para uso com o
toolkit `cbweb3`, demonstrando o cenário completo:

- **Dois spokes independentes**, cada um fundado pelo seu próprio banco central:
  - `spoke-brl` — fundado por `central-bank-brazil` (moeda BRL, chainId 1337)
  - `spoke-cop` — fundado por `central-bank-colombia` (moeda COP, chainId 1338)
- **Dois bancos comerciais por spoke**, cada um entrando via join bundle:
  - Brasil: `bank-itau`, `bank-bradesco` → `spoke-brl`
  - Colômbia: `bank-bancolombia`, `bank-davivienda` → `spoke-cop`

> Os nomes de banco são ilustrativos, apenas para exemplo de provisionamento.

Tudo é provisionado **por configuração** (manifesto YAML), sem editar código e
sem tocar na rede de referência (`deploy/local` + Makefile permanecem intactos).

Cobre as FASES 1B (toolkit `mode: found`) e 3 (`mode: join`). A **FASE 4**
(staging/prod: KMS real, CA real, imagens de registry) **não está implementada** —
por isso todos os manifestos usam `environment: local`, `keyProvider:
kms://local-emulator` e `certSource: self-signed`.

---

## Estrutura

```
samples/
  brazil/
    central-bank-brazil.yaml      # found  → spoke-brl
    bank-itau.yaml                # join   → spoke-brl
    bank-bradesco.yaml            # join   → spoke-brl
  colombia/
    central-bank-colombia.yaml    # found  → spoke-cop
    bank-bancolombia.yaml         # join   → spoke-cop
    bank-davivienda.yaml          # join   → spoke-cop
  bundles/                        # saída dos `apply` mode:found (não versionada)
```

## Matriz de portas (todos no mesmo host)

Cada nó Besu precisa de portas de host distintas. Esta é a alocação usada nos manifestos:

| Participante            | Spoke      | Modo  | RPC  | WS   | P2P   | chainId |
|-------------------------|------------|-------|------|------|-------|---------|
| central-bank-brazil     | spoke-brl  | found | 8645 | 8655 | 31303 | 1337    |
| bank-itau               | spoke-brl  | join  | 8646 | 8656 | 31304 | 1337    |
| bank-bradesco           | spoke-brl  | join  | 8647 | 8657 | 31305 | 1337    |
| central-bank-colombia   | spoke-cop  | found | 8745 | 8755 | 31403 | 1338    |
| bank-bancolombia        | spoke-cop  | join  | 8746 | 8756 | 31404 | 1338    |
| bank-davivienda         | spoke-cop  | join  | 8747 | 8757 | 31405 | 1338    |

---

## Pré-requisitos

- Go 1.26+, Docker + Docker Compose v2, `jq`, `openssl`, `curl`.
- Imagem `hyperledger/besu:25.8.0` disponível (puxada automaticamente no primeiro `up`).
- Diretório de dados gravável. Os manifestos usam um `spec.node.dataDir` **relativo**
  (`cbweb3-data/<participante>`), que o CLI resolve contra o diretório de trabalho
  atual (CWD) e cria automaticamente. Rodando `./deploy-all.sh` a partir de `samples/`,
  os dados e os join bundles ficam em `samples/cbweb3-data/` — sem `sudo` nem caminho
  privilegiado. Para usar outro local, edite `spec.node.dataDir` (relativo ou absoluto).

---

## Passo 0 — Compilar o toolkit

```bash
cd scenario-a/toolkit
go build -o ./cbweb3 ./cmd/cbweb3
```

O binário localiza a raiz do `scenario-a` automaticamente (busca por âncora a
partir do executável e do diretório atual), então funciona de **qualquer lugar
dentro do repositório** — inclusive rodando `./cbweb3` de dentro de `samples/`.
Se rodar o binário **fora** do repositório, aponte a raiz com `CBWEB3_HOME`:

```bash
export CBWEB3_HOME="$(cd ../ && pwd)"   # raiz do scenario-a
```

> Os templates (`provisioning/templates/...`) e scripts (`deploy/local/...`) são
> ativos canônicos do toolkit — não são copiados para `samples/`. O `deploy-contracts`
> roda `go test` nesses scripts, então a engine sempre requer o repositório presente.

Defina, para toda a sessão, o diretório onde os join bundles serão emitidos —
apontando para esta pasta `samples/`, de modo que os manifestos `mode: join`
encontrem o bundle em `../bundles/`:

```bash
export CBWEB3_OUTPUT_DIR="$(cd ../samples && pwd)"
CBWEB3="$(pwd)/cbweb3"
```

> Sem `CBWEB3_OUTPUT_DIR`, o bundle é gravado no diretório-pai do `dataDir`
> (com os manifestos de exemplo: `<CWD>/cbweb3-data/bundles/...`). Nesse caso, ajuste
> `joinBundleRef` nos manifestos de join para o caminho correspondente.

---

## Passo 1 — Validar os manifestos (dry-run)

O `--dry-run` valida schema, resolve o profile e mostra o plano de execução sem
executar nada. Rode em todos antes de provisionar:

```bash
for f in ../samples/brazil/*.yaml ../samples/colombia/*.yaml; do
  echo "== $f =="
  "$CBWEB3" apply -f "$f" --dry-run --output yaml
done
```

Erros de manifesto (campo ausente, valor inválido) são reportados de uma vez,
com mensagem clara, e o comando sai com código 1.

---

## Passo 2 — Subir o relay Cacti local

Os manifestos `mode: found` registram o spoke no relay, e `register-relay` é um
passo **obrigatório** (hard): o relay precisa estar no ar **antes** do `apply`.
Como a LNET não está disponível, suba um relay local:

```bash
../provisioning/scripts/start-cacti.sh
# espera o health em http://localhost:4000/api/v1/health
```

O relay expõe os endpoints de registro (RL-1): o `found` faz
`POST /api/v1/spokes` e o toolkit confirma via `GET /api/v1/spokes/<id>`. Os
manifestos de exemplo já apontam `spec.relay.endpoint: http://localhost:4000`.

---

## Passo 3 — Fundar o spoke do Brasil (`mode: found`)

```bash
"$CBWEB3" apply -f ../samples/brazil/central-bank-brazil.yaml --output yaml
```

O `found` é **CB-only**: cria a rede do país (banco central) a partir do
manifesto — sem subir o Besu manualmente e **sem** nós de banco fixos. A engine
executa a sequência idempotente de **9 passos**:

1. `start-besu` — sobe o nó Besu bootnode e **gera o genesis** na 1ª execução (idempotente; nunca regenera)
2. `deploy-contracts` — registry de nós Paladin, ZetoFactory, PenteFactory
3. `gen-tls` — cert TLS do nó Paladin do CB (self-signed)
4. `render-configs` — config do Paladin do CB
5. `register-nodes` — registra **o nó Paladin do CB** on-chain (lógica nativa, parametrizada por spoke — sem fallback `spoke-a`)
6. `start-paladin` — sobe o Paladin do CB + health-check
7. `create-zeto-token`
8. `onboard-registry` — deploya o `IdentityRegistry.sol` (whitelist de participantes) e registra o CB via chave de governança
9. `register-relay` — registra o spoke no relay Cacti (hard: falha se o relay não responder)

> O contexto **Pente** e o **FXAgreement** (bilaterais) **não** são criados no
> found — eles são criados no `join`, no relacionamento CB↔banco (pairwise).

Ao concluir, a rede do Brasil está **no ar e pronta para operar**, registrada no
relay e aguardando os bancos comerciais. É emitido o **join bundle**:

```
samples/bundles/spoke-brl.bundle.yaml
```

Ele contém o enode do bootnode, o genesis (hash + conteúdo), os endereços dos
contratos de nível-spoke (registry de nós, ZetoFactory, PenteFactory, ZetoToken,
e o whitelist de participantes), o conjunto de validadores QBFT, a CA do spoke
(âncora de confiança) e o `cbEndpoint`. **Não contém chaves privadas.**

> Idempotência: rodar `apply` de novo reexecuta apenas o que falta. O genesis
> **nunca** é regenerado em um spoke já existente.

---

## Passo 4 — Adicionar os bancos brasileiros (`mode: join`)

Com o bundle de `spoke-brl` emitido, provisione os dois bancos:

```bash
"$CBWEB3" apply -f ../samples/brazil/bank-itau.yaml     --output yaml
"$CBWEB3" apply -f ../samples/brazil/bank-bradesco.yaml --output yaml
```

A engine de join executa **15 passos**, em três blocos:

- **Entrada na rede Besu (1–8):** escreve o genesis do bundle, sobe o Besu
  sincronizando pelo enode do bootnode, aguarda sync, vota o validador QBFT, gera
  par de chaves + CSR (via `keyProvider`), envia o CSR ao CB (via `cbEndpoint`),
  recebe o cert assinado, faz proof-of-possession + registro no IdentityRegistry.
- **Paladin do banco, dinâmico (9–12):** `gen-tls-join` (cert do nó Paladin do
  banco, derivado de `bankId`), `render-config-join`, `start-paladin-join`
  (sobe o Paladin do banco), `register-paladin-node` (registra a identidade do
  nó on-chain — lógica nativa, sem nome de banco fixo).
- **Relacionamento privado CB↔banco (13–15):** `create-pente-context` (grupo
  Pente bilateral CB↔banco), `deploy-fxa-pente` (FXAgreement dentro do grupo),
  `start-backend`.

> **Pré-requisitos do join:**
> 1. O passo `request-cert` faz POST do CSR ao `cbEndpoint`. O **backend do banco
>    central (api-gateway)** precisa estar no ar, senão o join falha (`ErrCBUnreachable`).
> 2. Os passos de Pente exigem que **os dois nós Paladin (CB e banco) se enxerguem**
>    via transport mTLS na rede do spoke.
>
> A capacidade do toolkit subir automaticamente a stack de backend do CB e do
> banco é um próximo incremento (hoje o backend é um pré-requisito externo).

---

## Passo 5 — Fundar o spoke da Colômbia e adicionar seus bancos

Mesma sequência, manifestos da Colômbia. Como é um spoke independente, pode ser
feito em paralelo ou após o do Brasil (o relay Cacti do Passo 2 já serve os dois
spokes — cada um se registra com seu próprio id):

```bash
# Fundar spoke-cop
"$CBWEB3" apply -f ../samples/colombia/central-bank-colombia.yaml --output yaml
# → emite samples/bundles/spoke-cop.bundle.yaml

# Adicionar os bancos colombianos
"$CBWEB3" apply -f ../samples/colombia/bank-bancolombia.yaml --output yaml
"$CBWEB3" apply -f ../samples/colombia/bank-davivienda.yaml  --output yaml
```

---

## Verificação

Consulte o número do bloco em cada nó pela porta RPC (matriz acima):

```bash
for p in 8645 8646 8647 8745 8746 8747; do
  echo -n "porta $p: "
  curl -s -X POST "http://localhost:$p" \
    -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r .result
done
```

Containers em execução:

```bash
docker ps --filter "name=cbweb3-spoke-"
```

O relatório estruturado (`--output yaml|json`) do `apply` mostra o status de cada
passo (`success` / `skipped` / `failed` / `pending`).

---

## Observações

- **Relay Cacti local (RL-1).** Sem a LNET, suba o relay local com
  `provisioning/scripts/start-cacti.sh` (Passo 2). O `register-relay` é
  **obrigatório**: o `found` falha se o relay (`spec.relay.endpoint`) não
  responder. O registro é dinâmico via `POST /api/v1/spokes` e persiste no volume
  do relay. Observação: o registro habilita a **descoberta** do spoke; para que o
  relay também faça *polling* ativo de liquidação são necessários os dados
  completos de conexão (besuWs, grpcEndpoint, internalApiUrl) na config de polling
  do relay (`CACTI_SPOKES_CONFIG`) — isso é parte da generalização N-spokes (Fase 2).
- **FASE 4 (staging/prod) não implementada.** Apenas `environment: local` é aceito
  pelo CLI. As implementações prod de `keyProvider` (KMS real) e `certSource`
  (CA real) são stubs que retornam `ErrNotImplemented`. Promover para prod
  exigirá apenas valores diferentes no manifesto — sem mudanças na engine.
- **Endereçamento cross-stack.** `advertisedHost` é sempre explícito, nunca
  inferido de co-localização nem de IP de container (ver
  `provisioning/docs/adr-001-cross-stack-enode-addressing.md`). Em deploy local
  multi-stack, os participantes de um mesmo spoke compartilham a rede Docker
  `cbweb3-<spoke-id>-besu`.
- **Rede de referência intacta.** Este toolkit não modifica nem depende de
  `deploy/local` ou `make/*.mk` — eles continuam como rede de amostra.
