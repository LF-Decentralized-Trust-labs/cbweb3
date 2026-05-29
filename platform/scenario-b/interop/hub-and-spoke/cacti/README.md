# Cacti Liquidity Relay (Scenario B)

## Visão Geral

O Cacti Liquidity Relay é um serviço especializado para o **Scenario B** da plataforma CBWeb3, focado em pools de liquidez cooperativos baseados em AMM (Automated Market Maker).

Este serviço substitui o HTLC relay (Scenario A) e fornece funcionalidades específicas para:
- Observar eventos `CommitMatched` do contrato `LiquidityCommitRegistry`
- Notificar gateways quando commits de liquidez são matched
- Fornecer abstração Besu via Cacti PluginLedgerConnectorBesu

## Componentes

### LiquidityCommitWatcher
Observa o contrato `LiquidityCommitRegistry` no Hub e notifica os gateways configurados quando um evento `CommitMatched` é emitido. Isso permite que os Bancos Centrais adicionem liquidez soberana de forma autônoma quando um par é formado cooperativamente.

### Cacti PluginLedgerConnectorBesu
Fornece abstração para interagir com nodes Besu, disponibilizando:
- `getPastLogs` — buscar eventos históricos
- `getBlock` — obter informações de blocos
- `watchBlocksV1` — streaming de eventos via Socket.IO

## Configuração

### Variáveis de Ambiente Obrigatórias

```bash
# Spoke-A Besu connection
SPOKE_A_BESU_RPC=http://host.docker.internal:8645
SPOKE_A_BESU_WS=ws://host.docker.internal:8655

# Spoke-B Besu connection
SPOKE_B_BESU_RPC=http://host.docker.internal:8745
SPOKE_B_BESU_WS=ws://host.docker.internal:8755
```

### Variáveis de Ambiente Opcionais

```bash
# API port (default: 4000)
CACTI_API_PORT=4000

# Poll interval (default: 3000ms)
POLL_INTERVAL_MS=3000
```

### LiquidityCommitWatcher (Opcional mas Recomendado)

```bash
# LiquidityCommitRegistry contract address on Hub
LIQUIDITY_COMMIT_REGISTRY_ADDRESS=0xYourLCRAddressHere

# Hub Besu connection (if different from Spoke-A)
HUB_BESU_RPC=http://host.docker.internal:8645
HUB_BESU_WS=ws://host.docker.internal:8655

# Comma-separated list of gateway internal URLs
GATEWAY_INTERNAL_URLS=http://host.docker.internal:38080,http://host.docker.internal:60080

# Relay authentication secret
INTERNAL_RELAY_AUTH_SECRET=relay-secret-change-in-production

# Starting block number
LCR_WATCHER_START_BLOCK=0
```

## API Endpoints

### `GET /api/v1/health`

Endpoint de liveness/readiness.

**Response:**
```json
{
  "status": "ok",
  "uptime": 123.456,
  "mode": "scenario-b-liquidity",
  "watcher_active": true
}
```

### Cacti Connector Endpoints

Endpoints adicionais registrados pelo `PluginLedgerConnectorBesu`:
- `POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-past-logs`
- `POST /api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-block`
- Socket.IO em `/api/v1/plugins/socket.io/` para `watchBlocksV1`

Consulte a [documentação oficial do Cacti](https://github.com/hyperledger/cacti) para detalhes.

## Docker

### Build

```bash
docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml build
```

### Run

```bash
docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml up -d
```

### Logs

```bash
docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml logs -f
```

### Stop

```bash
docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml down
```

## Logs de Exemplo

### Startup Bem-Sucedido

```
Cacti Liquidity Relay starting (Scenario B)…
  Spoke-A RPC  : http://host.docker.internal:8645
  Spoke-A WS   : ws://host.docker.internal:8655
  Spoke-B RPC  : http://host.docker.internal:8745
  Spoke-B WS   : ws://host.docker.internal:8755
  API port     : 4000
[cacti] PluginLedgerConnectorBesu spoke-a initialized (besu-connector-spoke-a-abc123)
[cacti] PluginLedgerConnectorBesu spoke-b initialized (besu-connector-spoke-b-def456)
[cacti] LiquidityCommitWatcher started
[cacti] spoke-a registered 3 web service endpoint(s)
[cacti] spoke-b registered 3 web service endpoint(s)
Cacti Liquidity Relay API listening on :4000
```

### Watcher Não Configurado

```
[cacti] LiquidityCommitWatcher not configured — relay will be idle
```

Isso ocorre quando `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` ou `GATEWAY_INTERNAL_URLS` não estão configurados. O serviço ainda funciona como abstração Besu, mas não observa eventos de liquidez.

## Diferenças do Scenario A

O Scenario A (HTLC-based) foi **completamente removido** desta implementação. Se você precisa de suporte a HTLC, use uma versão anterior ou crie um serviço separado.

**Funcionalidades removidas:**
- HTLC Relay (observação de `LogHTLCLocked` e `LogHTLCClaimed`)
- FX Agreement coordination
- Endpoints `/api/v1/relay/events/settle`, `/api/v1/relay/events/lock`, etc.
- Relay proof storage
- gRPC calls para `SettleHTLC` no payment-orchestrator

**Funcionalidades mantidas/adicionadas:**
- LiquidityCommitWatcher (Scenario B)
- Cacti PluginLedgerConnectorBesu (ambos scenarios)
- Endpoint `/api/v1/health` com informações do modo

## Troubleshooting

### Watcher não inicia

**Sintoma:**
```
[LiquidityCommitWatcher] LIQUIDITY_COMMIT_REGISTRY_ADDRESS not set — watcher disabled
```

**Solução:** Configure `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` com o endereço do contrato LCR.

### Gateway retorna 401/403

**Sintoma:**
```
[LiquidityCommitWatcher] gateway http://host:38080 failed: HTTP 401
```

**Solução:** Verifique se `INTERNAL_RELAY_AUTH_SECRET` está corretamente configurado em ambos, Cacti e gateways.

### Não observa eventos

**Sintoma:** Nenhum log de `CommitMatched` mesmo após commits serem feitos.

**Solução:**
1. Verifique se `LIQUIDITY_COMMIT_REGISTRY_ADDRESS` está correto
2. Confirme conectividade com Hub Besu (`HUB_BESU_RPC` e `HUB_BESU_WS`)
3. Verifique se `LCR_WATCHER_START_BLOCK` não está após os blocos com eventos

## Desenvolvimento

### Estrutura de Arquivos

```
interop/hub-and-spoke/cacti/
├── src/
│   ├── index.ts                       # Entry point principal
│   ├── config.ts                      # Carregador de configuração
│   └── liquidity-commit-watcher.ts    # Watcher de eventos LCR
├── docker-compose.yaml                # Deploy via Docker
├── Dockerfile                         # Build da imagem
├── env-sample                         # Exemplo de .env
├── package.json                       # Dependências Node.js
└── README.md                          # Este arquivo
```

### Testes Locais

```bash
# Instalar dependências
npm install

# Configurar variáveis de ambiente
cp env-sample .env
# Editar .env com valores reais

# Executar
npm start
```

### Build Local

```bash
npm run build
node dist/index.js
```

## Referências

- [Scenario B Documentation](../../../specs/005-cooperative-liquidity/)
- [LiquidityCommitRegistry Contract](../../../contracts/src/interfaces/ILiquidityCommitRegistry.sol)
- [Hyperledger Cacti](https://github.com/hyperledger/cacti)
- [Integration Guide](../../../docs-reference/integracao-scenario-b-new.md)
