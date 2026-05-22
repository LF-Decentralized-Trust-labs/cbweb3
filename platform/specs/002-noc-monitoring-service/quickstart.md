# Quickstart: NOC Monitoring Service

**Branch**: `002-noc-monitoring-service`

---

## Prerequisites

- Docker + Docker Compose
- Go 1.25.5
- Platform already running (Keycloak, Besu, Cacti relay, Paladin)

---

## 1. Start NOC Backend + Database

```bash
cd interop/hub-and-spoke/noc/

# Copy and fill env
cp .env.example .env
# Set: KEYCLOAK_URL, DB_PASSWORD, NOC_BACKEND_PORT (default: 8090)

docker compose up -d
```

The backend runs migrations on startup. Verify:
```bash
curl http://localhost:8090/api/v1/health
# → {"status":"ok","uptime":5}
```

---

## 2. Register Keycloak Roles and NOC Client

```bash
# Create the noc-backend Keycloak client and roles:
./deploy/local/keycloak/setup-noc-client.sh

# Create your first NOC operator user:
./deploy/local/keycloak/create-noc-user.sh --username noc-operator1 --role noc-operator
```

---

## 3. Register Spokes

```bash
# Get an admin token
TOKEN=$(./deploy/local/keycloak/get-noc-token.sh --role noc-admin)

# Register Spoke-A
curl -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"spoke-a","currency_code":"BRL","jurisdiction":"Brazil"}'

# Register Spoke-B
curl -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"spoke-b","currency_code":"ARS","jurisdiction":"Argentina"}'
```

---

## 4. Configure and Start Agents

Each agent needs a `agent.yaml`. Examples are in `interop/hub-and-spoke/noc/agent-configs/`.

```bash
# Add the noc-agent service to Spoke-A's docker-compose:
# interop/hub-and-spoke/cacti/docker-compose.yaml
#
#   noc-agent-spoke-a:
#     image: cbweb3/noc-agent:local
#     volumes:
#       - /var/run/docker.sock:/var/run/docker.sock
#       - ./noc-agent.yaml:/app/agent.yaml:ro
#     environment:
#       - NOC_AGENT_KEY=${NOC_AGENT_KEY_SPOKE_A}
#     restart: unless-stopped

docker compose -f interop/hub-and-spoke/cacti/docker-compose.yaml up -d noc-agent-spoke-a
```

Verify agent is pushing:
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/v1/overview
# → should show spoke-a components with HEALTHY/OFFLINE status
```

---

## 5. Build Services Locally

```bash
# Agent
cd backend/services/noc-agent
go build ./cmd/agent/...

# Backend
cd backend/services/noc-backend
go build ./cmd/server/...
```

## 6. Run Tests

```bash
cd backend/services/noc-agent && go test ./...
cd backend/services/noc-backend && go test ./...
```

---

## Environment Variables

### `noc-backend`

| Variable | Default | Description |
|----------|---------|-------------|
| `NOC_BACKEND_PORT` | `8090` | HTTP port |
| `DATABASE_URL` | — | PostgreSQL DSN |
| `KEYCLOAK_URL` | — | e.g., `http://keycloak:8080` |
| `KEYCLOAK_REALM` | `cbweb3` | Keycloak realm |
| `KEYCLOAK_CLIENT_ID` | `noc-backend` | |
| `JWKS_CACHE_TTL_SECONDS` | `300` | JWKS cache TTL |
| `SLO_WORKER_INTERVAL_SECONDS` | `300` | SLO recompute interval |
| `LOG_BUFFER_MAX_LINES` | `1000` | Max log lines per component |
| `AGENT_GRACE_PERIOD_MULTIPLIER` | `3` | Grace periods before UNKNOWN |

### `noc-agent`

| Variable | Default | Description |
|----------|---------|-------------|
| `NOC_AGENT_KEY` | — | API key (must match backend registration) |
| `AGENT_CONFIG_PATH` | `/app/agent.yaml` | Path to agent.yaml |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker socket |
