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
- Diretório de dados gravável. Os manifestos usam `/opt/cbweb3/data/<participante>`.
  Crie-o com permissão para o seu usuário, ou ajuste `spec.node.dataDir` em cada manifesto:

  ```bash
  sudo mkdir -p /opt/cbweb3/data && sudo chown "$(id -u):$(id -g)" /opt/cbweb3/data
  ```

---

## Passo 0 — Compilar o toolkit

```bash
cd scenario-a/toolkit
go build -o ./cbweb3 ./cmd/cbweb3
```

O binário resolve caminhos de templates e scripts relativos à sua localização.
Mantenha-o em `scenario-a/toolkit/cbweb3` (como acima) para que os defaults de
template (`provisioning/templates/...`) sejam encontrados.

Defina, para toda a sessão, o diretório onde os join bundles serão emitidos —
apontando para esta pasta `samples/`, de modo que os manifestos `mode: join`
encontrem o bundle em `../bundles/`:

```bash
export CBWEB3_OUTPUT_DIR="$(cd ../samples && pwd)"
CBWEB3="$(pwd)/cbweb3"
```

> Sem `CBWEB3_OUTPUT_DIR`, o bundle é gravado no diretório-pai do `dataDir`
> (`/opt/cbweb3/data/bundles/...`). Nesse caso, ajuste `joinBundleRef` nos
> manifestos de join para o caminho absoluto correspondente.

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

O `found` agora **cria a rede inteira do país a partir do manifesto** — não é
mais necessário subir o Besu manualmente. A engine executa a sequência
idempotente de 11 passos:

1. `start-besu` — sobe o nó Besu bootnode (compose TK-4) e **gera o genesis** na 1ª execução (idempotente; nunca regenera)
2. `deploy-contracts` — IdentityRegistry, ZetoFactory, PenteFactory, FXAgreement
3. `gen-tls` — TLS via `certSource` (self-signed; o CB gera a CA do spoke)
4. `render-configs` — configs do Paladin
5. `register-nodes` — registro dos nós Paladin
6. `start-paladin` — start + health-check
7. `create-zeto-token`
8. `create-pente-context`
9. `deploy-fxa-pente` — FXAgreement dentro do contexto Pente
10. `onboard-registry` — onboarding real no IdentityRegistry (não o atalho)
11. `register-relay` — registra o spoke no relay Cacti (hard: falha se o relay não responder)

Ao concluir, a rede do Brasil está **no ar e pronta para operar**, registrada no
relay e aguardando os bancos comerciais. É emitido o **join bundle**:

```
samples/bundles/spoke-brl.bundle.yaml
```

Ele contém o enode do bootnode, o genesis (hash + conteúdo), os endereços dos
contratos, o conjunto de validadores QBFT, a CA do spoke (âncora de confiança) e
o `cbEndpoint`. **Não contém chaves privadas.**

> Idempotência: rodar `apply` de novo reexecuta apenas o que falta. O genesis
> **nunca** é regenerado em um spoke já existente.

---

## Passo 4 — Adicionar os bancos brasileiros (`mode: join`)

Com o bundle de `spoke-brl` emitido, provisione os dois bancos:

```bash
"$CBWEB3" apply -f ../samples/brazil/bank-itau.yaml     --output yaml
"$CBWEB3" apply -f ../samples/brazil/bank-bradesco.yaml --output yaml
```

A engine de join executa 9 passos: escreve o genesis do bundle, sobe o Besu
sincronizando pelo enode do bootnode, aguarda sync, vota o validador QBFT, gera
par de chaves + CSR (via `keyProvider`), envia o CSR ao CB (via `cbEndpoint`),
recebe o cert assinado, faz proof-of-possession + registro no IdentityRegistry e
sobe o backend do banco.

> **Pré-requisito do join:** o passo `request-cert` faz POST do CSR ao
> `cbEndpoint` do bundle. O backend do banco central (api-gateway, modo smart de
> onboarding) precisa estar no ar e acessível nesse endpoint, senão o join falha
> rápido (`ErrCBUnreachable`).

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
