# Quickstart: TK-6 — Emissor do Join Bundle

Este guia mostra como usar `EmitBundle` programaticamente após o engine TK-5 completar o provisionamento de um spoke `mode: found`.

## Pré-requisitos

1. TK-5 (`RunFound`) completou com sucesso — todos os 10 passos em `done` em `.provisioning-state.yaml`.
2. Besu do spoke está rodando e acessível em `BesuRPCURL`.
3. `SPOKE_DATA_DIR` contém:
   - `genesis/genesis.json`
   - `.deployed-addrs.env` com todos os 7 endereços
   - `tls/central-bank.crt`

---

## Uso em Go (integrado ao TK-7)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func main() {
    ctx := context.Background()

    // Manifesto do banco central (já validado por TK-1)
    m := &manifest.Manifest{
        APIVersion: "cbweb3/v1",
        Kind:       "ParticipantDeployment",
        Metadata:   manifest.Metadata{Name: "central-bank-brazil"},
        Spec: manifest.Spec{
            Scenario: "a",
            Mode:     "found",
            Spoke:    manifest.Spoke{ID: "spoke-brl", ChainID: 1337, Currency: "BRL"},
            Node: manifest.Node{
                AdvertisedHost: "cbweb3-spoke-brl-besu.central-bank-brazil",
                P2P:            &manifest.Port{Port: 31303},
            },
            Relay: &manifest.Relay{Endpoint: "http://cbweb3-cacti:4000"},
        },
    }

    dataDir   := "/var/cbweb3/spokes/spoke-brl"  // SPOKE_DATA_DIR
    outputDir := "."                               // bundles/ criado aqui

    // BesuEnodeProvider faz JSON-RPC admin_nodeInfo no Besu
    ep := bundle.NewBesuEnodeProvider("http://localhost:8645", nil)

    jb, err := bundle.EmitBundle(ctx, bundle.BundleInput{
        Manifest:      m,
        DataDir:       dataDir,
        OutputDir:     outputDir,
        EnodeProvider: ep,
    })
    if err != nil {
        log.Fatalf("emit bundle: %v", err)
    }

    fmt.Printf("bundle emitido: %s/bundles/%s.bundle.yaml\n", outputDir, jb.Spec.SpokeID)
    fmt.Printf("  enode:   %s\n", jb.Spec.Bootnode.Enode)
    fmt.Printf("  genesis: %s\n", jb.Spec.Genesis.Hash)
}
```

---

## Verificação manual do bundle

```bash
# Verificar existência e estrutura
cat bundles/spoke-brl.bundle.yaml

# Confirmar apiVersion e kind
grep -E "^(apiVersion|kind):" bundles/spoke-brl.bundle.yaml
# Esperado:
# apiVersion: cbweb3/v1
# kind: JoinBundle

# Verificar enode (deve conter o advertisedHost do manifesto)
grep "enode:" bundles/spoke-brl.bundle.yaml

# Verificar hash do genesis (deve coincidir com sha256sum do arquivo original)
HASH=$(grep "hash:" bundles/spoke-brl.bundle.yaml | awk '{print $2}' | sed 's/sha256://')
EXPECTED=$(sha256sum /var/cbweb3/spokes/spoke-brl/genesis/genesis.json | awk '{print $1}')
[ "$HASH" = "$EXPECTED" ] && echo "✓ genesis hash ok" || echo "✗ genesis hash MISMATCH"

# Garantir ausência de chaves privadas (OBRIGATÓRIO antes de distribuir)
grep "PRIVATE KEY" bundles/spoke-brl.bundle.yaml && echo "ERRO: chave privada no bundle!" || echo "✓ sem chaves privadas"

# Decodificar e validar genesis embutido
grep "content:" bundles/spoke-brl.bundle.yaml | awk '{print $2}' | base64 -d | python3 -m json.tool > /dev/null && echo "✓ genesis JSON válido"
```

---

## Erros comuns

| Erro | Causa | Solução |
|---|---|---|
| `bundle: EmitBundle requires mode: found` | Manifesto tem `mode: join` | Usar manifesto de banco central com `mode: found` |
| `bundle: genesis.json not found in dataDir` | TK-5 não completou ou `dataDir` incorreto | Verificar passo 1 do engine (template TK-4 cria genesis) |
| `bundle: deployed-addrs.env missing required key: ZETO_TOKEN_ADDRESS` | Passos 6–8 do TK-5 não completaram | Re-executar TK-5; verificar `.provisioning-state.yaml` |
| `bundle: CA cert not found or invalid in dataDir/tls` | Passo 2 do TK-5 não completou | Verificar `<dataDir>/tls/central-bank.crt` |
| `bundle: could not obtain enode from Besu` | Besu não está rodando ou `BesuRPCURL` incorreto | Verificar `docker compose ps` e a URL do RPC |

---

## Rodar testes unitários

```bash
cd scenario-a/toolkit
go test -race ./engine/bundle/...

# Com testes de integração (requer artefatos reais)
go test -race -tags integration -timeout 5m ./engine/bundle/...
```

---

## Distribuição do bundle

O arquivo `bundles/spoke-brl.bundle.yaml` é público e pode ser:

- Commitado no repositório de configuração do spoke
- Revisado em PR antes de distribuição
- Enviado diretamente ao operador do banco comercial (por e-mail seguro, S3, etc.)
- Utilizado como `joinBundleRef` no manifesto `mode: join` do banco comercial:

```yaml
# manifesto do banco comercial
apiVersion: cbweb3/v1
kind: ParticipantDeployment
spec:
  mode: join
  joinBundleRef: ./bundles/spoke-brl.bundle.yaml
```

**Não contém chaves privadas**: o bundle é seguro para distribuição pública dentro do consórcio.
