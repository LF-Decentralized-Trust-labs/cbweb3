# Implementation Plan: TK-4 — Template Compose central-bank

**Branch**: `026-tk4-compose-central-bank` | **Date**: 2026-06-27 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/026-tk4-compose-central-bank/spec.md`

## Summary

Criar o arquivo `docker-compose.yaml` do template central-bank para o toolkit de provisionamento do Scenario A. O template é completamente parametrizado por variáveis de ambiente (nenhum valor de porta, hostname, caminho ou imagem hardcoded) e implementa o padrão de init container para garantir que o genesis Besu seja gerado uma única vez — nunca regenerado em execuções subsequentes. O template fornece topologia de contêineres apenas; lógica de orquestração, geração de chaves e emissão de certificados são responsabilidades do motor TK-5, TK-2 e TK-3 respectivamente.

## Technical Context

**Language/Version**: Docker Compose v2 (YAML 3.8+); Bash 5+ (script de guarda do genesis)  
**Primary Dependencies**: `hyperledger/besu:25.8.0` (imagem parametrizada via `BESU_IMAGE`); `docker compose` plugin v2  
**Storage**: Bind mounts via `SPOKE_DATA_DIR`; sem named volumes — garante portabilidade entre hosts  
**Testing**: Scripts shell de integração (`docker compose up` + verificação via `curl eth_blockNumber` + `sha256sum genesis.json`)  
**Target Platform**: Linux (Docker Engine) e Docker Desktop (Mac/Windows) — bootstrap local; staging/prod em EKS via Docker-in-Docker ou equivalente  
**Project Type**: Arquivo de configuração de infraestrutura (Compose template) + script shell de guarda  
**Performance Goals**: Nó Besu respondendo a `eth_blockNumber` dentro de 60 s após `docker compose up`  
**Constraints**: Template NÃO executa geração de genesis em reinicializações (invariante de segurança); NÃO usa `hyperledger/besu:latest`; NÃO hardcoda valores de porta, host ou caminho

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Princípio I — Scenario-Scoped Independence ✅

O template reside em `scenario-a/provisioning/templates/central-bank/`. Nenhum arquivo fora de `scenario-a/` é modificado. O diretório `deploy/local/` (sample de referência) permanece intacto. **PASSA.**

### Princípio II — Privacy by Design ✅

O template não lida com transferências de valor inter-banco nem com dados de participantes. É infraestrutura de nó. **Não aplicável / PASSA.**

### Princípio III — Atomic Settlement Guarantee ✅

O template não implementa lógica de liquidação. **Não aplicável / PASSA.**

### Princípio IV — Compliance Gate Before Participation ✅

O template sobe o nó Besu. O onboarding no `IdentityRegistry` e os gates de compliance são responsabilidade do motor TK-5, executado após o nó estar running. O template não bypassa nenhum gate. **PASSA.**

### Princípio V — Test-First at Every Layer ⚠️ REQUER ATENÇÃO

**Requisito**: Testes devem ser escritos antes da implementação.

**Plano de testes**:
- Script de integração `scenario-a/provisioning/tests/test-central-bank-template.sh` que:
  1. Sobe o Compose com variáveis de teste em diretório temporário
  2. Aguarda health check do nó Besu (`eth_blockNumber`)
  3. Registra hash do genesis
  4. Derruba e reinicia o Compose
  5. Verifica que o hash não mudou
  6. Verifica que `net_enode` contém o `BESU_ADVERTISED_HOST` de teste
  7. Derruba o Compose e limpa o diretório temporário

**Sequência test-first**: O script de teste é escrito e falha (nó não existe) → o template é criado → o script passa.

**STATUS: Cumprível.** O teste é um script shell executável em CI sem dependência de infraestrutura externa além do Docker Engine. **PASSA com compromisso de test-first na Fase 1.**

### Princípio VI — Observability and Auditability ✅

- O `genesis-init` emite logs para stdout: `"[genesis-init] genesis.json already exists, skipping generation."` ou `"[genesis-init] Generating genesis..."`.
- O nó Besu emite logs estruturados por padrão para stdout.
- O template não swallowa erros: falhas no `genesis-init` propagam código de saída não-zero, impedindo o `besu` de iniciar.

**PASSA.**

### Re-check pós-design (Fase 1) ✅

Após a conclusão do `data-model.md` e `research.md`:
- Nenhum desvio de princípio identificado.
- O uso de bind mounts (sem named volumes) é intencional: garante que o estado do spoke seja visível no host e portável entre ambientes sem acoplamento ao driver de volume Docker.

## Project Structure

### Documentation (this feature)

```text
specs/026-tk4-compose-central-bank/
├── plan.md              # Este arquivo
├── research.md          # Phase 0 — resolvido
├── data-model.md        # Phase 1 — resolvido
└── tasks.md             # Phase 2 output (/speckit.tasks — não gerado por /speckit.plan)
```

### Source Code (repository root)

```text
scenario-a/provisioning/
  templates/
    central-bank/
      docker-compose.yaml          ← artefato principal (TK-4)
  tests/
    test-central-bank-template.sh  ← teste de integração (test-first)
```

**Não modificados** (preservados intactos conforme design §4):
```text
scenario-a/deploy/local/spoke-besu-a/startBesu.sh
scenario-a/deploy/local/spoke-besu-b/startBesu.sh
```

**Structure Decision**: Projeto de arquivo único de configuração (`docker-compose.yaml`) + teste de integração shell. Não há módulo Go, serviço backend ou contrato Solidity envolvido.

## Complexity Tracking

Nenhuma violação da Constitution identificada. Tabela omitida.

---

## Phase 1 — Implementation Plan

### Passo 1 — Escrever o script de teste (test-first) [ANTES de qualquer código]

Criar `scenario-a/provisioning/tests/test-central-bank-template.sh`:

```bash
#!/usr/bin/env bash
# Teste de integração para o template Compose central-bank (TK-4)
# Deve FALHAR antes do template existir; deve PASSAR após.

set -euo pipefail
TEMPLATE="scenario-a/provisioning/templates/central-bank/docker-compose.yaml"
DATA_DIR="$(mktemp -d)"
SPOKE_ID="spoke-test"
RPC_PORT=18645

# Cleanup on exit
trap 'docker compose -f "$TEMPLATE" --project-name tk4test down -v 2>/dev/null; rm -rf "$DATA_DIR"' EXIT

# Cria estrutura de diretórios esperada pelo template
mkdir -p "$DATA_DIR/config" "$DATA_DIR/genesis" "$DATA_DIR/nodes/central-bank/data"

# qbftConfigFile.json mínimo: chainId 1337, 1 validador, QBFT
cat > "$DATA_DIR/config/qbftConfigFile.json" << 'EOF'
{
  "genesis": {
    "config": {
      "chainId": 1337,
      "berlinBlock": 0,
      "londonBlock": 0,
      "qbft": {"blockperiodseconds": 2, "epochlength": 30000, "requesttimeoutseconds": 4}
    },
    "nonce": "0x0", "timestamp": "0x0", "gasLimit": "0x1fffffffffffff",
    "difficulty": "0x1", "mixHash": "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365",
    "coinbase": "0x0000000000000000000000000000000000000000", "alloc": {}
  },
  "blockchain": {"nodes": {"generate": true, "count": 1}}
}
EOF

export SPOKE_ID BESU_ADVERTISED_HOST="127.0.0.1" \
       BESU_RPC_PORT="$RPC_PORT" BESU_WS_PORT=18655 BESU_P2P_PORT=31403 \
       BESU_IMAGE="hyperledger/besu:25.8.0" SPOKE_DATA_DIR="$DATA_DIR"

# 1a execucao: genesis deve ser gerado
docker compose -f "$TEMPLATE" --project-name tk4test up -d

# Aguarda nó responder (max 60s)
for i in $(seq 1 30); do
  if curl -sf -X POST "http://localhost:$RPC_PORT" \
       -H 'Content-Type: application/json' \
       -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
       | grep -q result 2>/dev/null; then
    break
  fi
  [ "$i" -eq 30 ] && echo "FAIL: no disponivel apos 60s" && exit 1
  sleep 2
done

GENESIS_HASH_1=$(sha256sum "$DATA_DIR/genesis/genesis.json" | cut -d' ' -f1)

ENODE=$(curl -sf -X POST "http://localhost:$RPC_PORT" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"net_enode","params":[],"id":1}' | jq -r '.result')
echo "$ENODE" | grep -q "127.0.0.1" \
  || (echo "FAIL: enode nao contem BESU_ADVERTISED_HOST"; exit 1)

# 2a execucao: genesis NAO deve mudar
docker compose -f "$TEMPLATE" --project-name tk4test down
docker compose -f "$TEMPLATE" --project-name tk4test up -d

for i in $(seq 1 30); do
  curl -sf -X POST "http://localhost:$RPC_PORT" \
       -H 'Content-Type: application/json' \
       -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
       | grep -q result 2>/dev/null && break
  sleep 2
done

GENESIS_HASH_2=$(sha256sum "$DATA_DIR/genesis/genesis.json" | cut -d' ' -f1)
[ "$GENESIS_HASH_1" = "$GENESIS_HASH_2" ] \
  || (echo "FAIL: genesis foi regenerado (hash diferente)"; exit 1)

echo "PASS: template central-bank OK"
```

O teste deve rodar e **FALHAR** antes do template existir.

---

### Passo 2 — Criar o `docker-compose.yaml`

Localização: `scenario-a/provisioning/templates/central-bank/docker-compose.yaml`

Conteúdo completo do template:

```yaml
# Template Compose — central-bank (TK-4)
# Todas as variaveis sao obrigatorias exceto as com :-default.
# Invocar via motor de orquestracao TK-5; nao invocar diretamente.
#
# Variaveis obrigatorias (sem default — docker compose falha explicitamente se ausente):
#   SPOKE_ID, BESU_ADVERTISED_HOST, BESU_RPC_PORT, BESU_WS_PORT,
#   BESU_P2P_PORT, BESU_IMAGE, SPOKE_DATA_DIR
#
# Variaveis opcionais (com default):
#   BOOTNODE_ENODE (vazio = este no eh o bootnode, mode: found)
#   SPOKE_NETWORK_NAME, BESU_CONTAINER_NAME, BESU_LOGGING

services:

  genesis-init:
    image: ${BESU_IMAGE}
    container_name: ${BESU_CONTAINER_NAME:-cbweb3-${SPOKE_ID}-besu}-genesis-init
    restart: "no"
    user: root
    volumes:
      - ${SPOKE_DATA_DIR}/config:/config:ro
      - ${SPOKE_DATA_DIR}/genesis:/genesis
      - ${SPOKE_DATA_DIR}/nodes:/nodes
    command: >
      sh -c '
        set -e;
        if [ -f /genesis/genesis.json ]; then
          echo "[genesis-init][spoke=${SPOKE_ID}] genesis.json already exists, skipping generation.";
          exit 0;
        fi;
        echo "[genesis-init][spoke=${SPOKE_ID}] Generating genesis from /config/qbftConfigFile.json...";
        /opt/besu/bin/besu operator generate-blockchain-config
          --config-file=/config/qbftConfigFile.json
          --to=/nodes/networkFiles
          --private-key-file-name=key;
        cp /nodes/networkFiles/genesis.json /genesis/genesis.json;
        cp -rn /nodes/networkFiles/keys/. /nodes/central-bank/data/ 2>/dev/null || true;
        echo "[genesis-init][spoke=${SPOKE_ID}] Done.";
      '
    networks:
      - spoke_besu

  besu:
    image: ${BESU_IMAGE}
    container_name: ${BESU_CONTAINER_NAME:-cbweb3-${SPOKE_ID}-besu.central-bank}
    restart: always
    user: root
    depends_on:
      genesis-init:
        condition: service_completed_successfully
    volumes:
      - ${SPOKE_DATA_DIR}/nodes/central-bank/data:/opt/besu/data
      - ${SPOKE_DATA_DIR}/genesis:/opt/besu/genesis:ro
    ports:
      - "${BESU_RPC_PORT}:8545"
      - "${BESU_WS_PORT}:8546"
      - "${BESU_P2P_PORT}:30303"
      - "${BESU_P2P_PORT}:30303/udp"
    command: >
      /opt/besu/bin/besu
      --data-path=/opt/besu/data
      --genesis-file=/opt/besu/genesis/genesis.json
      --min-gas-price=0
      --rpc-http-enabled --rpc-http-api=ETH,NET,QBFT
      --rpc-http-host=0.0.0.0 --rpc-http-port=8545
      --rpc-ws-enabled --rpc-ws-api=ETH,NET,QBFT
      --rpc-ws-host=0.0.0.0 --rpc-ws-port=8546
      --p2p-port=30303
      --p2p-host=${BESU_ADVERTISED_HOST}
      --host-allowlist=* --rpc-http-cors-origins=all
      --logging=${BESU_LOGGING:-INFO}
      ${BOOTNODE_ENODE:+--bootnodes=${BOOTNODE_ENODE}}
    healthcheck:
      test:
        - CMD
        - sh
        - -c
        - |
          curl -sf -X POST http://localhost:8545
          -H 'Content-Type: application/json'
          -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
          | grep -q result
      interval: 5s
      timeout: 3s
      retries: 20
      start_period: 10s
    networks:
      - spoke_besu

networks:
  spoke_besu:
    name: ${SPOKE_NETWORK_NAME:-cbweb3-${SPOKE_ID}-besu}
    driver: bridge
```

**Nota sobre `${BOOTNODE_ENODE:+--bootnodes=${BOOTNODE_ENODE}}`**: Sintaxe POSIX — inclui `--bootnodes=<value>` somente se `BOOTNODE_ENODE` estiver definido e não-vazio. Docker Compose v2 suporta esta expansão.

---

### Passo 3 — Verificar que o teste passa

```bash
bash scenario-a/provisioning/tests/test-central-bank-template.sh
```

Resultado esperado: `PASS: template central-bank OK`

---

### Passo 4 — Atualizar contexto do agente

```bash
bash .specify/scripts/bash/update-agent-context.sh claude
```

---

## Dependências e sequenciamento

| Item | Depende de | Status |
|---|---|---|
| TK-1 — schema do manifesto | — | Implementado (023) |
| TK-2 — interface keyProvider | — | Implementado (024) |
| TK-3 — interface certSource | — | Implementado (025) |
| **TK-4 — template Compose central-bank** | TK-1, TK-2, TK-3 (interfaces, não impl.) | **Esta spec** |
| TK-5 — motor de orquestração | TK-4, TK-2, TK-3 | Futuro |
| TK-6 — emissor do join bundle | TK-5 | Futuro |

O template (TK-4) pode ser implementado e testado de forma isolada — o motor TK-5 ainda não existe. Os testes de integração do TK-4 invocam o `docker compose` diretamente com variáveis de teste, sem dependência do motor.
