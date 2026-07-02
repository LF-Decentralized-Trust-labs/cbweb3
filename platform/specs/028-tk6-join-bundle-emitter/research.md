# Research: TK-6 — Emissor do Join Bundle

## R-01 — Como obter o enode via `admin_nodeInfo`

**Pergunta**: Como extrair o enode-id do bootnode de forma confiável sem depender do host/porta que o Besu anuncia internamente?

**Resultado**: O Besu expõe o método JSON-RPC `admin_nodeInfo` (disponível por padrão no RPC HTTP). Exemplo de chamada e resposta:

```bash
curl -s -X POST http://localhost:8645 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'
```

Resposta:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "enode": "enode://deadbeef1234...@0.0.0.0:30303",
    "id": "deadbeef1234...",
    "ip": "0.0.0.0",
    "listenAddr": "0.0.0.0:30303",
    "name": "besu/v25.8.0/linux-x86_64/openjdk-java-21",
    "ports": { "discovery": 30303, "listener": 30303 }
  }
}
```

**Estratégia de parsing do enode**:
1. Extrair `result.enode` da resposta JSON.
2. Localizar o separador `@`: `strings.LastIndex(enode, "@")`.
3. Tudo antes de `@` é `enode://<id>` — preservar.
4. Substituir o sufixo `@<host>:<port>` por `@<manifest.advertisedHost>:<manifest.p2pPort>`.

**Formato canônico**: `enode://<512-bit-pubkey-hex>@<host>:<port>`

**Casos edge**:
- Host `0.0.0.0` ou `[::]`: sempre substituir — nunca é um endereço routável externo.
- Host já correto (ambientes onde Besu anuncia o host externo): substituição ainda é feita incondicionalmente — o manifesto é a fonte de verdade de endereçamento.
- `@` ausente: enode mal formado → `ErrEnodeUnavailable`.

**Decisão**: Substituição incondicional de host e porta. O enode-id (chave pública) vem do Besu; endereço vem do manifesto. Não há heurística de "se parece IP interno, substitui".

---

## R-02 — Embedding vs referência para o genesis

**Pergunta**: Deve o bundle embutir o genesis.json (base64) ou apenas referenciar por hash+path?

**Análise**:

| Abordagem | Vantagem | Desvantagem |
|---|---|---|
| **Embedding (base64)** | Bundle auto-contido; TK-9 não precisa buscar genesis separadamente | Bundle maior; genesis redundante se versionado |
| **Referência (hash + path)** | Bundle menor; genesis centralizado | TK-9 precisa acesso ao path ou a um endpoint separado |

**Tamanho real**: Genesis Besu com QBFT tem ~3–8 KB (contas pré-fundadas, configuração de validadores). Base64 adiciona ~33% → ~4–11 KB no bundle. Completamente aceitável em YAML.

**Decisão**: **Embedding** (base64 RFC 4648 sem quebra de linha, campo `spec.genesis.content`). O bundle é self-contained — TK-9 não precisa de canal lateral para obter o genesis. O hash `spec.genesis.hash` permite verificação de integridade pelo consumidor sem re-decodificar.

**Limite documentado**: Genesis acima de 1 MB emite um warning no log mas não bloqueia o emissor. Na prática isso nunca ocorre com Besu.

---

## R-03 — Validação do cert PEM

**Pergunta**: Como garantir que `tls/central-bank.crt` é um certificado X.509 válido e não contém chave privada?

**Resposta**: Usar `encoding/pem` da stdlib:

```go
block, rest := pem.Decode(certBytes)
if block == nil {
    return ErrCACertNotFound // PEM inválido ou ausente
}
if block.Type != "CERTIFICATE" {
    return fmt.Errorf("%w: expected CERTIFICATE block, got %s", ErrCACertNotFound, block.Type)
}
if strings.Contains(string(certBytes), "PRIVATE KEY") {
    return fmt.Errorf("%w: cert file contains private key material", ErrCACertNotFound)
}
```

**Por que verificar `PRIVATE KEY` no conteúdo total e não apenas no primeiro bloco**: O arquivo pode conter múltiplos blocos PEM (cert + key concatenados). A busca em `string(certBytes)` garante que nenhum bloco de chave privada escape.

**Não fazer**: `x509.ParseCertificate` para validar o cert. O bundle apenas transmite o PEM como âncora de confiança para TK-9; validade criptográfica completa é responsabilidade do consumidor.

---

## R-04 — Escrita atômica do bundle

**Pergunta**: Como evitar que TK-9 leia um bundle parcialmente escrito?

**Resposta**: Padrão temp+rename, idêntico ao já usado em `state.go` do TK-5:

```go
tmp, err := os.CreateTemp(bundleDir, ".bundle-*.yaml.tmp")
// ... escrever conteúdo ...
tmp.Close()
os.Rename(tmp.Name(), bundlePath)
```

`os.Rename` é atômico em Linux (mesma partição). O arquivo de bundle visível para leitores nunca está em estado parcial — ou não existe, ou está completo.

**Isolamento concorrente**: Dois processos emitindo o bundle concorrentemente resultam em um vencedor (o último a renomear). O conteúdo de qualquer emissão é sempre completo; não há corrupção.

---

## R-05 — Integração com TK-7 (`cbweb3 apply`)

**Pergunta**: Quem chama `EmitBundle` e quando?

**Resposta**: TK-7 é o orquestrador de alto nível:

```
cbweb3 apply -f manifest.yaml
  → valida manifesto (TK-1)
  → RunFound(ctx, manifest, deps)    [TK-5]
  → EmitBundle(ctx, BundleInput{...}) [TK-6]  ← chamado após RunFound retornar nil
  → reporta caminho do bundle ao operador
```

TK-6 **não** é chamado pelo engine TK-5 internamente. A separação preserva a coesão: TK-5 orquestra os passos de provisionamento; TK-6 é um passo de saída de artefato. TK-7 é o único que conhece ambos.

**Consequência**: `EmitBundle` não verifica `.provisioning-state.yaml` — não é responsabilidade do emissor saber se o spoke foi totalmente provisionado. Se chamado sobre um `dataDir` parcialmente provisionado, os erros de artefatos ausentes (FR-005, FR-006, FR-007) são suficientes.

---

## R-06 — Reutilização de `parseDeployedAddrs`

**Pergunta**: `parseDeployedAddrs` está em `engine/orchestrator/addrs.go` (package `orchestrator`). TK-6 precisa da mesma função. Como evitar duplicação?

**Opções**:

| Opção | Prós | Contras |
|---|---|---|
| **Mover para `engine/addrs/`** | Sem duplicação; pacote compartilhado natural | Requer alterar `orchestrator` (import path muda) — pequeno refactor em TK-5 |
| **Re-exportar em `orchestrator`** | Sem mover arquivos | `bundle` dependeria de `orchestrator` — acoplamento indesejado |
| **Duplicar em `bundle`** | Zero acoplamento | Dois lugares para manter |

**Decisão**: **Mover para `engine/addrs/`**. É uma mudança simples (renomear package, atualizar import em `orchestrator`). O pacote `addrs` é compartilhado por qualquer componente que precise ler `.deployed-addrs.env`. Isso é explicitado como um pré-requisito da tarefa T001 (refactoring de addrs).

---

## R-07 — `EnodeProvider` como interface injetável

**Pergunta**: Como testar `EmitBundle` sem uma instância real do Besu?

**Resposta**: A interface `EnodeProvider` (data-model.md) permite injetar um stub nos testes:

```go
type staticEnodeProvider struct{ enode string }
func (p *staticEnodeProvider) NodeInfo(_ context.Context) (string, error) { return p.enode, nil }
```

Isso espelha o padrão de `RelayRegistrar` em TK-5: a implementação HTTP real é `BesuEnodeProvider`; os testes usam um stub. Toda a lógica de parsing, substituição de host, e composição do bundle é testável sem I/O de rede.

---

## R-08 — Encoding base64 sem quebra de linha

**Pergunta**: Qual variante de base64 usar para `spec.genesis.content`?

**Resposta**: `base64.StdEncoding.EncodeToString(genesisBytes)` — RFC 4648 standard alphabet, sem padding alternativo, sem newlines (diferente de `base64.StdEncoding` que não insere newlines por padrão em Go). O YAML é mais legível sem quebras de linha no campo base64; o consumidor (TK-9) usa `base64.StdEncoding.DecodeString`.

**Não usar**: `base64.RawStdEncoding` (sem padding `=`). Padding explícito facilita debugging manual (`base64 -d`).

---

## R-09 — Formato do campo `genesis.hash`

**Convenção**: `sha256:<hex-lowercase>` — consistente com OCI image digest e Docker content-addressable storage. Permite ao consumidor validar o genesis sem decodificar o base64:

```bash
echo -n "sha256:" && sha256sum <(base64 -d <<< "$content") | awk '{print $1}'
```

Ou diretamente: `sha256sum genesis.json | awk '{print "sha256:" $1}'`.

---

## Decisões consolidadas

| # | Decisão |
|---|---|
| R-01 | Enode: substituição incondicional de host e porta pelo manifesto |
| R-02 | Genesis: embedding base64, self-contained, limit-warn em > 1 MB |
| R-03 | CA cert: validar bloco PEM + busca de `PRIVATE KEY` em todo o conteúdo |
| R-04 | Escrita atômica: temp + rename (igual ao state.go do TK-5) |
| R-05 | Caller: TK-7, não TK-5 — separação de responsabilidade |
| R-06 | `parseDeployedAddrs` movida para `engine/addrs/` (pacote compartilhado) |
| R-07 | `EnodeProvider` interface injetável, `BesuEnodeProvider` como impl padrão |
| R-08 | base64 StdEncoding sem newlines para genesis content |
| R-09 | Hash format: `sha256:<hex-lowercase>` |
