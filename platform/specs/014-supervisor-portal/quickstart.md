# Quickstart: Supervisor Portal (014)

## Prerequisites

- Full stack running: `make scenario-b.up` (or `scenario-a.up`)
- A Keycloak user with `ROLE_SUPERVISOR` assigned to the `cbweb3` client role
- `VITE_API_BASE_URL` set in the supervisor app's `.env` (e.g. `http://localhost:8080`)

## Backend: Test new endpoints manually

```bash
# 1. Obtain a supervisor session token (replace with your KC credentials)
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"supervisor@centralbank.gov","password":"supervisor123"}' \
  | jq -r '.access_token')

# 2. Fetch audit logs (supervisor route)
curl -s "http://localhost:8080/api/v1/compliance/audit/logs?severity=CRITICAL&limit=10" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 3. Verify a ZK Pointer (Scenario B only)
curl -s "http://localhost:8080/api/v1/compliance/zk-pointer/verify?bank_id=cb_spoke_a&commitment_hash=0xabc123" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4. Open a disclosure request
curl -s -X POST http://localhost:8080/api/v1/oversight/disclosure-request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tx_ref":"0x1234...","requestor_id":"cb_lnet","reason_code":"AML_ALERT"}' | jq .

# 5. Sign the disclosure request (use request_id from step 4)
curl -s -X POST http://localhost:8080/api/v1/oversight/disclosure-sign \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"<id-from-step-4>","signer_id":"cb_spoke_a"}' | jq .

# 6. Poll status
curl -s "http://localhost:8080/api/v1/oversight/disclosure-status/<id-from-step-4>" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

## Frontend: Run supervisor app

```bash
# Scenario B
cd scenario-b/frontend
VITE_API_BASE_URL=http://localhost:8080 pnpm --filter supervisor dev

# Scenario A
cd scenario-a/frontend
VITE_API_BASE_URL=http://localhost:8080 pnpm --filter supervisor dev
```

Open `http://localhost:5174` (default Vite port for supervisor).

## Verify no mocks are used

In browser DevTools → Network tab, confirm all requests go to `localhost:8080` and none to a local mock handler. No requests should be intercepted by `mock-db.ts`.

## Run backend tests

```bash
# Scenario B — supervisor handler + ZK pointer
cd scenario-b/backend/services/api-gateway
go test ./internal/http/handlers/... -run TestSupervisorHandler -v

# Scenario A — oversight service + handler
cd scenario-a/backend/services/compliance
go test ./internal/services/... -run TestOversightService -v
cd scenario-a/backend/services/api-gateway
go test ./internal/http/handlers/... -run TestOversightHandler -v
```

## Run E2E tests

```bash
# Scenario B
cd scenario-b/tests
pnpm playwright test supervisor/audit-vault.spec.ts

# Scenario A
cd scenario-a/tests
pnpm playwright test supervisor/audit-vault.spec.ts
```
