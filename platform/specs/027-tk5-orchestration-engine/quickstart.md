# Quickstart: TK-5 — Motor de orquestração (`mode: found`)

**Feature**: `027-tk5-orchestration-engine`

---

## Pré-requisitos

1. **TK-2 implementado**: `toolkit/engine/keyprovider/` com `LocalKeyProvider`
2. **TK-3 implementado**: `toolkit/engine/certsource/` com `LocalCertSource`
3. **TK-4 implementado**: `provisioning/templates/central-bank/docker-compose.yaml` com genesis-init
4. **Besu rodando**: o nó Besu do spoke deve estar em execução antes de invocar o engine
5. **Go 1.26+**, **Docker Compose v2**, **`curl`**, **`jq`** disponíveis no host

---

## Fluxo típico de um operador

```bash
# 1. Subir o nó Besu com o template TK-4
export SPOKE_ID=spoke-brl
export BESU_RPC_PORT=8645
export BESU_WS_PORT=8655
export BESU_P2P_PORT=31303
export BESU_IMAGE=hyperledger/besu:25.8.0
export SPOKE_DATA_DIR=/data/spokes/spoke-brl
export BESU_ADVERTISED_HOST=cbweb3-spoke-brl-besu.central-bank-brazil
export SPOKE_NETWORK_NAME=cbweb3-spoke-brl-besu

mkdir -p "${SPOKE_DATA_DIR}"
docker compose -f scenario-a/provisioning/templates/central-bank/docker-compose.yaml up -d
# Aguardar Besu health check (eth_blockNumber)

# 2. Invocar o engine via CLI apply (TK-7) — ou diretamente em Go
# cbweb3 apply -f manifests/central-bank-brazil.yaml

# O engine:
# [1] Deploy contracts (IdentityRegistry, ZetoFactory, PenteFactory) — ~90s
# [2] Gen TLS certs via CertSource — <1s
# [3] Render Paladin configs — <1s
# [4] Register Paladin nodes — ~30s
# [5] Stop/clean/start Paladin + health check — ~3-5min
# [6] Create Zeto token — ~30s
# [7] Create Pente bilateral context — ~30s
# [8] Deploy FXAgreement in Pente — ~30s
# [9] Onboard central bank in IdentityRegistry — ~15s
# [10] Register spoke in relay — <5s (or skip if relay unavailable)

# 3. Verificar o resultado
curl -sS -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"ptx_getTransaction","params":["dummy"]}' \
  http://localhost:31648 | grep PD020704

# 4. Re-executar (idempotente): completa em < 2s, todos os passos pulados
# cbweb3 apply -f manifests/central-bank-brazil.yaml
```

---

## Uso programático (Go)

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/certsource"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
    "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

func main() {
    m, err := manifest.ParseFile("manifests/central-bank-brazil.yaml")
    if err != nil {
        log.Fatal(err)
    }

    kp, _ := keyprovider.New("kms://local-emulator")
    cs, _ := certsource.New("self-signed://local")

    deps := orchestrator.Deps{
        KeyProvider:             kp,
        CertSource:              cs,
        RelayRegistrar:          orchestrator.NoOpRelayRegistrar{}, // relay RL-1 pending
        ScriptsDir:              "scenario-a/deploy/local/paladin/scripts",
        ComposeTemplatePath:     "scenario-a/provisioning/templates/central-bank/paladin-compose.yaml",
        PaladinConfigTemplateDir: "scenario-a/provisioning/templates/central-bank/paladin-config",
        BesuRPCURL:              "http://localhost:8645",
        PaladinCBURL:            "http://localhost:31648",
    }

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    if err := orchestrator.RunFound(ctx, m, deps); err != nil {
        log.Fatalf("provisioning failed: %v", err)
    }
}
```

---

## Executar os testes (test-first)

```bash
cd scenario-a/toolkit

# Testes unitários (sem Docker, sem Besu — mocks de todas as dependências externas)
go test -race ./engine/orchestrator/... -run TestUnit -v

# Testes de integração (requer Besu + Docker disponíveis)
go test -race ./engine/orchestrator/... -run TestIntegration -v -timeout 15m

# Cobertura
go test ./engine/orchestrator/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Arquivo de manifesto de referência

```yaml
# manifests/central-bank-brazil.yaml
apiVersion: cbweb3/v1
kind: ParticipantDeployment
metadata:
  name: central-bank-brazil
spec:
  scenario: a
  environment: local
  role: central-bank
  mode: found
  spoke:
    id: spoke-brl
    chainId: 1337
    currency: BRL
  node:
    rpc:   { port: 8645 }
    ws:    { port: 8655 }
    p2p:   { port: 31303 }
    advertisedHost: cbweb3-spoke-brl-besu.central-bank-brazil
    dataDir: /data/spokes/spoke-brl
  image: hyperledger/besu:25.8.0
  keyProvider: kms://local-emulator
  certSource: self-signed://local
  relay:
    endpoint: http://cbweb3-cacti:4000
```

---

## Arquivo de estado após execução bem-sucedida

```yaml
# /data/spokes/spoke-brl/.provisioning-state.yaml
spokeID: spoke-brl
steps:
  - step: deploy-contracts
    status: done
    completedAt: "2026-06-27T14:32:45Z"
  - step: gen-tls
    status: done
    completedAt: "2026-06-27T14:32:46Z"
  - step: render-configs
    status: done
    completedAt: "2026-06-27T14:32:47Z"
  - step: register-nodes
    status: done
    completedAt: "2026-06-27T14:33:20Z"
  - step: start-paladin
    status: done
    completedAt: "2026-06-27T14:38:05Z"
  - step: create-zeto-token
    status: done
    completedAt: "2026-06-27T14:38:38Z"
  - step: create-pente-context
    status: done
    completedAt: "2026-06-27T14:39:12Z"
  - step: deploy-fxa-pente
    status: done
    completedAt: "2026-06-27T14:39:45Z"
  - step: onboard-registry
    status: done
    completedAt: "2026-06-27T14:40:02Z"
  - step: register-relay
    status: done
    completedAt: "2026-06-27T14:40:08Z"
```
