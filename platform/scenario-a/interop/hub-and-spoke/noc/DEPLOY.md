# NOC Portal — Local Setup Guide

This guide covers the complete local setup of the NOC stack: building images,
configuring Keycloak, registering spokes in the database, provisioning agent
API keys, and starting the agents.

**Prerequisite:** `make spoke-all` must have completed successfully — Keycloak,
both Besu networks, Paladin nodes, and the Cacti relay must be running.

---

## Step 1 — Build the images

From the `scenario-a/` root:

```bash
docker compose -f interop/hub-and-spoke/noc/docker-compose.yaml build
```

This builds three local images: `cbweb3/noc-backend:local`, `cbweb3/noc-agent:local`,
and `cbweb3/noc-portal:local`.

---

## Step 2 — Configure the environment

```bash
cd interop/hub-and-spoke/noc/
cp .env.example .env
```

The defaults in `.env.example` work for local dev without changes. The NOC
client in Keycloak is public (no client secret), so `KEYCLOAK_CLIENT_SECRET`
can stay empty.

---

## Step 3 — Set up Keycloak

From the `scenario-a/` root:

```bash
make noc.setup-keycloak
```

This creates (idempotent — safe to rerun):
- Realm `cbweb3` (if not already present)
- Client `noc-portal` — public, ROPC enabled
- Roles: `ROLE_NOC_VIEWER`, `ROLE_NOC_OPERATOR`, `ROLE_NOC_ADMIN`
- Default user: `noc-admin` / `noc-admin` with the `ROLE_NOC_ADMIN` role

---

## Step 4 — Start backend and database

```bash
cd interop/hub-and-spoke/noc/
docker compose up -d noc-backend noc-db
```

Wait for the health check to pass:

```bash
curl http://localhost:8090/api/v1/health
# {"status":"ok","uptime":3}
```

---

## Step 5 — Register spokes and provision API keys

This step populates the `noc-db` with the spoke records and the agent API keys
that the agents use to authenticate their metric pushes.

```bash
# Get an admin token from Keycloak
TOKEN=$(curl -s -X POST "http://localhost:8081/realms/cbweb3/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=noc-admin&password=noc-admin&grant_type=password&client_id=noc-portal" \
  | jq -r '.access_token')

# Register Spoke-A
SPOKE_A_ID=$(curl -s -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"spoke-a","currency_code":"BRL","jurisdiction":"Brazil"}' \
  | jq -r '.id')
echo "Spoke-A ID: $SPOKE_A_ID"

# Register Spoke-B
SPOKE_B_ID=$(curl -s -X POST http://localhost:8090/api/v1/admin/spokes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"spoke-b","currency_code":"ARS","jurisdiction":"Argentina"}' \
  | jq -r '.id')
echo "Spoke-B ID: $SPOKE_B_ID"

# Provision API key for the Spoke-A agent
curl -s -X POST http://localhost:8090/api/v1/admin/agents/provision-key \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"raw_key":"noc-agent-spoke-a-key-local","spoke_id":"'"$SPOKE_A_ID"'","hint":"agent-spoke-a"}'

# Provision API key for the Spoke-B agent
curl -s -X POST http://localhost:8090/api/v1/admin/agents/provision-key \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"raw_key":"noc-agent-spoke-b-key-local","spoke_id":"'"$SPOKE_B_ID"'","hint":"agent-spoke-b"}'
```

---

## Step 6 — Update agent configs with the spoke IDs

The `agent.yaml` files are mounted read-only into the agent containers. Update
the `spoke_id` field in each one with the IDs obtained above:

```bash
# agent-configs/spoke-a/agent.yaml  →  spoke_id: "<SPOKE_A_ID>"
# agent-configs/spoke-b/agent.yaml  →  spoke_id: "<SPOKE_B_ID>"
```

The `api_key` values (`noc-agent-spoke-a-key-local` and
`noc-agent-spoke-b-key-local`) match what was provisioned in Step 5 — no
change needed there.

---

## Step 7 — Start the agents

```bash
docker compose up -d noc-agent-spoke-a noc-agent-spoke-b
```

Verify the agents are pushing successfully (no 401 errors):

```bash
docker logs noc-agent-spoke-a --tail 20
docker logs noc-agent-spoke-b --tail 20
# Expected: "push successful"
```

---

## Step 8 — Run the frontend

```bash
cd frontend/apps/noc/
npm run dev
# Listening on http://localhost:5900
```

Login: `noc-admin` / `noc-admin`

Frontend env vars (create `frontend/apps/noc/.env.local` if needed):

```env
VITE_KEYCLOAK_URL=http://localhost:8081
VITE_KEYCLOAK_REALM=cbweb3
VITE_KEYCLOAK_CLIENT_ID=noc-portal
VITE_NOC_BACKEND_URL=http://localhost:8090
```

---

## Verification

```bash
# Network overview — shows all spokes and component health
curl -H "Authorization: Bearer $TOKEN" http://localhost:8090/api/v1/overview | jq \
  '.spokes[] | {name, components: [.components[] | {name, type, health_status}]}'
```

Each component should show `HEALTHY` after the first agent push cycle (~15 s).

---

## Teardown

```bash
docker compose down          # stop containers, keep noc-db-data volume
docker compose down -v       # stop containers and delete the database volume
```

---

## Troubleshooting

### Agents returning 401

The `spoke_id` in `agent.yaml` doesn't match any registered spoke, or the API
key wasn't provisioned. Re-run Step 5, then update the agent configs and
restart the agents:

```bash
docker compose restart noc-agent-spoke-a noc-agent-spoke-b
```

### Components showing UNKNOWN

The agent can't reach the component endpoint. Verify the component containers
are running and that the agent is connected to the right Docker networks
(`spoke_a_besu_network`, `spoke_b_besu_network`, `cacti_default`).

### noc-backend fails to start

Check that Keycloak is reachable from inside the container:

```bash
docker exec noc-backend curl -s http://host.docker.internal:8081/realms/cbweb3 | jq .realm
# "cbweb3"
```

If the realm doesn't exist, rerun `make noc.setup-keycloak`.

### Starting fresh (reset spoke registration)

```bash
docker compose down -v       # wipe the database
docker compose up -d noc-backend noc-db
# then redo Steps 5–7
```
