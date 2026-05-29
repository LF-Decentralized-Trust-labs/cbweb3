# Guia de Deploy: NOC Portal (Apenas Imagens Docker)

## Pré-requisitos

- Docker + Docker Compose rodando
- Keycloak já configurado e acessível
- Imagens Docker do NOC já buildadas:
  - `cbweb3/noc-backend:local`
  - `cbweb3/noc-agent:local`
  - `cbweb3/noc-portal:local` (frontend)

---

## PARTE 1: Configurar o Stack NOC (Backend + DB)

### 1.1. Configurar variáveis de ambiente

```bash
cd interop/hub-and-spoke/noc/
cp .env.example .env
```

Edite o `.env` e preencha:

```bash
# ── PostgreSQL (noc-db) ──────────────────────────────────────────────────────
POSTGRES_USER=noc
POSTGRES_PASSWORD=sua_senha_segura_aqui
POSTGRES_DB=nocdb

# ── noc-backend ──────────────────────────────────────────────────────────────
NOC_BACKEND_PORT=8090

# Keycloak integration
KEYCLOAK_URL=http://host.docker.internal:8081  # ou o URL interno do seu Keycloak
KEYCLOAK_REALM=cbweb3
KEYCLOAK_CLIENT_ID=noc-portal
KEYCLOAK_CLIENT_SECRET=  # deixe em branco por enquanto, vamos pegar no próximo passo

# Frontend origins (ajuste conforme suas portas)
NOC_FRONTEND_ORIGIN=http://localhost:5173,http://localhost:5900

# Grace period multiplier
AGENT_GRACE_MULTIPLIER=3

# Dev only - NUNCA habilite em produção
NOC_SKIP_AUTH=false
```

### 1.2. Obter o Keycloak Client Secret

Execute o script para obter o client secret do Keycloak (caso o realm `cbweb3` e o client `noc-portal` já existam):

```bash
# Método direto - executar comando dentro do container do Keycloak
docker exec cbweb3-keycloak bash -c "
  /opt/keycloak/bin/kcadm.sh config credentials \
    --server http://localhost:8080 \
    --realm master \
    --user admin \
    --password admin > /dev/null 2>&1
  
  CLIENT_ID=\$(/opt/keycloak/bin/kcadm.sh get clients -r cbweb3 \
    --fields id,clientId | jq -r '.[] | select(.clientId==\"noc-portal\") | .id')
  
  /opt/keycloak/bin/kcadm.sh get clients/\$CLIENT_ID/client-secret -r cbweb3 | jq -r '.value'
"
```

**OU** se o realm NOC ainda não foi criado, execute o setup completo:

```bash
./deploy/local/keycloak/setup-noc-realm.sh
```

Isso criará:
- Realm `cbweb3` (se não existir)
- Client `noc-portal` (público, ROPC habilitado)
- Roles: `noc-viewer`, `noc-operator`, `noc-admin`
- Usuário padrão: `noc-admin` / `noc-admin`

Copie o client secret e adicione no arquivo `.env`:

```bash
KEYCLOAK_CLIENT_SECRET=cole_o_secret_aqui
```

### 1.3. Iniciar o stack NOC (backend + banco)

```bash
cd interop/hub-and-spoke/noc/
docker compose up -d noc-backend noc-db
```

Verificar se subiu corretamente:

```bash
curl http://localhost:8090/api/v1/health
# Deve retornar: {"status":"ok","uptime":5}
```

---

## PARTE 2: Registrar Spokes no NOC Backend

### 2.1. Obter token de admin do Keycloak

```bash
# Login como noc-admin
TOKEN=$(curl -s -X POST "http://localhost:8081/realms/cbweb3/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=noc-admin" \
  -d "password=noc-admin" \
  -d "grant_type=password" \
  -d "client_id=noc-portal" \
  | jq -r '.access_token')

echo $TOKEN  # Verifica se pegou o token
```

### 2.2. Registrar os Spokes

```bash
# Registrar Spoke-A (BRL - Brazil)
SPOKE_A_ID=$(curl -s -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "spoke-a",
    "currency_code": "BRL",
    "jurisdiction": "Brazil"
  }' | jq -r '.id')

echo "Spoke-A ID: $SPOKE_A_ID"

# Registrar Spoke-B (ARS - Argentina)
SPOKE_B_ID=$(curl -s -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "spoke-b",
    "currency_code": "ARS",
    "jurisdiction": "Argentina"
  }' | jq -r '.id')

echo "Spoke-B ID: $SPOKE_B_ID"
```

**Importante:** Guarde esses IDs, você vai precisar deles para provisionar as API keys dos agents.

---

## PARTE 3: Provisionar API Keys dos Agents

Cada agent precisa de uma API key provisionada que é vinculada a um spoke específico.

### 3.1. Gerar API keys para cada spoke

```bash
# Provisionar key para Spoke-A
curl -X POST http://localhost:8090/api/v1/admin/agents/provision-key \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "raw_key": "noc-agent-spoke-a-key-local",
    "spoke_id": "'"$SPOKE_A_ID"'",
    "hint": "agent-spoke-a"
  }'

# Provisionar key para Spoke-B
curl -X POST http://localhost:8090/api/v1/admin/agents/provision-key \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "raw_key": "noc-agent-spoke-b-key-local",
    "spoke_id": "'"$SPOKE_B_ID"'",
    "hint": "agent-spoke-b"
  }'
```

---

## PARTE 4: Configurar os Agents

### 4.1. Criar os arquivos de configuração dos agents

Crie o arquivo `agent-configs/spoke-a/agent.yaml`:

```yaml
spoke_id: "COLE_AQUI_O_SPOKE_A_ID"
noc_backend_url: "http://noc-backend:8090"
api_key: "noc-agent-spoke-a-key-local"
push_interval_seconds: 15

components:
  # Besu nodes
  - name: "besu-central-bank-a"
    type: "BESU"
    endpoint: "http://cbweb3-spoke-a-besu.central-bank-a:8545"
    container_name: "cbweb3-spoke-a-besu.central-bank-a"

  - name: "besu-bank-a"
    type: "BESU"
    endpoint: "http://cbweb3-spoke-a-besu.bank-a:8545"
    container_name: "cbweb3-spoke-a-besu.bank-a"

  - name: "besu-bank-c"
    type: "BESU"
    endpoint: "http://cbweb3-spoke-a-besu.bank-c:8545"
    container_name: "cbweb3-spoke-a-besu.bank-c"

  # Paladin nodes
  - name: "paladin-central-bank-a"
    type: "PALADIN"
    endpoint: "http://paladin-spoke-a-cb:8548"
    container_name: "paladin-spoke-a-cb"

  - name: "paladin-bank-a"
    type: "PALADIN"
    endpoint: "http://paladin-spoke-a-bank-a:8548"
    container_name: "paladin-spoke-a-bank-a"

  - name: "paladin-bank-c"
    type: "PALADIN"
    endpoint: "http://paladin-spoke-a-bank-c:8548"
    container_name: "paladin-spoke-a-bank-c"

  # Cacti relay
  - name: "cacti-relay"
    type: "CACTI_RELAY"
    endpoint: "http://cbweb3-cacti-relay:4000"
    container_name: "cbweb3-cacti-relay"
```

Faça o mesmo para `agent-configs/spoke-b/agent.yaml` (ajustando os nomes de containers e endpoints conforme seu ambiente).

### 4.2. Iniciar os agents

```bash
cd interop/hub-and-spoke/noc/
docker compose up -d noc-agent-spoke-a noc-agent-spoke-b
```

Verificar logs:

```bash
docker logs noc-agent-spoke-a -f
docker logs noc-agent-spoke-b -f
```

---

## PARTE 5: Verificar Funcionamento

### 5.1. Verificar overview do NOC

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/v1/overview | jq
```

Você deve ver:
- `spokes`: lista com spoke-a e spoke-b
- `components`: lista de componentes monitored (besu, paladin, cacti-relay)
- `health_status`: status de cada componente (HEALTHY, DEGRADED, OFFLINE, UNKNOWN)

### 5.2. Acessar o Portal NOC (Frontend)

Se você tiver o frontend rodando localmente:

```bash
cd frontend/apps/noc/
npm run dev
```

Configurar `.env.local`:

```bash
VITE_KEYCLOAK_URL=http://localhost:8081
VITE_KEYCLOAK_REALM=cbweb3
VITE_KEYCLOAK_CLIENT_ID=noc-portal
VITE_NOC_BACKEND_URL=http://localhost:8090
```

Acesse: http://localhost:5173

Login: `noc-admin` / `noc-admin`

---

## PARTE 6: Troubleshooting

### Agent não está enviando dados

1. Verificar logs do agent:
```bash
docker logs noc-agent-spoke-a
```

2. Verificar se o agent consegue alcançar o backend:
```bash
docker exec noc-agent-spoke-a curl http://noc-backend:8090/api/v1/health
```

3. Verificar se a API key está correta:
```bash
# No backend, verificar os logs de auth
docker logs noc-backend | grep -i "auth\|agent"
```

### Backend não está autenticando

1. Verificar se o KEYCLOAK_URL está correto no `.env`
2. Verificar se o client secret está correto
3. Testar token manualmente:
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/v1/admin/spokes
```

### Componentes aparecem como UNKNOWN

1. Verificar se os endpoints estão corretos no `agent.yaml`
2. Verificar se os containers dos componentes estão rodando
3. Verificar se o agent tem acesso às redes Docker corretas (spoke_a_besu_network, cacti_default, etc.)

---

## Resumo dos Endpoints

| Endpoint | Descrição |
|----------|-----------|
| `GET /api/v1/health` | Health check (sem auth) |
| `GET /api/v1/overview` | Overview completo (requer token) |
| `POST /api/v1/admin/spokes` | Registrar spoke (requer noc-admin) |
| `POST /api/v1/admin/agents/provision-key` | Provisionar API key para agent (requer noc-admin) |
| `GET /api/v1/alerts` | Listar alertas ativos |
| `GET /api/v1/slo` | Métricas SLO |

---

## Variáveis de Ambiente Importantes

### Backend (.env)

- `DATABASE_URL`: Connection string do PostgreSQL
- `KEYCLOAK_URL`: URL do Keycloak (interno ou externo)
- `KEYCLOAK_CLIENT_SECRET`: **OBRIGATÓRIO** - Secret do client noc-portal
- `NOC_SKIP_AUTH`: **NUNCA** habilite em produção

### Agent (agent.yaml)

- `spoke_id`: **OBRIGATÓRIO** - UUID do spoke registrado
- `api_key`: **OBRIGATÓRIO** - Key provisionada no backend
- `noc_backend_url`: URL do backend NOC
- `push_interval_seconds`: Intervalo de push (padrão: 15s)
