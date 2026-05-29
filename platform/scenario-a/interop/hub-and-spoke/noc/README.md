# NOC Monitoring Stack

Sistema de monitoramento centralizado para o CBWeb3 Platform, monitora componentes da infraestrutura (Besu, Paladin, Cacti Relay) através de agents distribuídos.

## 📚 Documentação

- **[DEPLOY.md](./DEPLOY.md)** - Guia completo de deployment usando apenas imagens Docker
  - Configuração do backend e banco de dados
  - Setup do Keycloak (client secret, roles, usuários)
  - Registro de spokes no NOC
  - Provisionamento de API keys para agents
  - Configuração e inicialização dos agents
  - Troubleshooting

## 🚀 Quick Start

```bash
# 1. Configurar ambiente
cd interop/hub-and-spoke/noc/
cp .env.example .env
# Edite o .env com suas credenciais

# 2. Iniciar stack
docker compose up -d

# 3. Verificar health
curl http://localhost:8090/api/v1/health
```

Para setup completo com registro de spokes e agents, consulte [DEPLOY.md](./DEPLOY.md).

## 🏗️ Arquitetura

```
┌─────────────────┐
│   NOC Portal    │  (Frontend React + Vite)
│  (Port 5173)    │
└────────┬────────┘
         │
         │ HTTP + JWT
         ▼
┌─────────────────┐
│  NOC Backend    │  (Go + Fiber + GORM)
│  (Port 8090)    │
└────────┬────────┘
         │
         ├─────────► PostgreSQL (noc-db)
         │
         └─────────► Keycloak (Auth)
         
         ▲
         │ Push Metrics
         │ (X-Agent-Key)
         │
    ┌────┴────┬────────┐
    │         │        │
┌───┴───┐ ┌──┴───┐ ┌──┴───┐
│Agent-A│ │Agent-B│ │Agent-C│
└───┬───┘ └──┬───┘ └──┬───┘
    │        │        │
  Spoke-A  Spoke-B  Hub
```

## 📦 Componentes

- **noc-backend**: API REST + Workers (Go)
- **noc-db**: PostgreSQL 15
- **noc-agent**: Agent de monitoramento por spoke/hub
- **noc-portal**: Interface web (frontend)

## 🔐 Autenticação e Autorização

### Roles

| Role | Permissões |
|------|-----------|
| `noc-viewer` | Visualizar dashboards, alertas, logs |
| `noc-operator` | `noc-viewer` + resolver alertas |
| `noc-admin` | `noc-operator` + gerenciar spokes e agents |

### Usuário Padrão

Após executar `setup-noc-realm.sh`:
- Username: `noc-admin`
- Password: `noc-admin`
- Roles: `noc-admin`

## 📊 Endpoints Principais

| Endpoint | Auth | Descrição |
|----------|------|-----------|
| `GET /api/v1/health` | Público | Health check |
| `GET /api/v1/overview` | JWT | Overview completo da rede |
| `POST /api/v1/admin/spokes` | noc-admin | Registrar spoke |
| `POST /api/v1/admin/agents/provision-key` | noc-admin | Provisionar API key |
| `GET /api/v1/alerts` | noc-viewer | Listar alertas |
| `GET /api/v1/components/{id}/logs` | noc-viewer | Logs de componente |

## 📝 Arquivos de Configuração

### `.env` (Backend)

```bash
POSTGRES_USER=noc
POSTGRES_PASSWORD=your_password
POSTGRES_DB=nocdb
NOC_BACKEND_PORT=8090
KEYCLOAK_URL=http://keycloak:8080
KEYCLOAK_REALM=cbweb3
KEYCLOAK_CLIENT_ID=noc-portal
KEYCLOAK_CLIENT_SECRET=your_secret
```

### `agent.yaml` (Agent)

```yaml
spoke_id: "uuid-do-spoke"
noc_backend_url: "http://noc-backend:8090"
api_key: "api-key-provisionada"
push_interval_seconds: 15

components:
  - name: "besu-node"
    type: "BESU"
    endpoint: "http://besu:8545"
    container_name: "cbweb3-besu"
```

## 🛠️ Development

### Build Local

```bash
# Backend
cd backend/services/noc-backend
go build ./cmd/server

# Agent
cd backend/services/noc-agent
go build ./cmd/agent

# Frontend
cd frontend/apps/noc
npm run build
```

### Testes

```bash
# Backend
cd backend/services/noc-backend && go test ./...

# Agent
cd backend/services/noc-agent && go test ./...
```

## 📚 Documentação Adicional

- [Spec Completa](../../../specs/002-noc-monitoring-service/spec.md)
- [Data Model](../../../specs/002-noc-monitoring-service/data-model.md)
- [API Contracts](../../../specs/002-noc-monitoring-service/contracts/rest-api.md)
- [Quickstart](../../../specs/002-noc-monitoring-service/quickstart.md)
