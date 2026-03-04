# API Gateway (CBWeb3)

This service is a Go + Fiber API gateway that handles:
- client authentication (`/auth/login`);
- bearer token validation for protected endpoints;
- wallet-to-user binding with Ethereum signature verification;
- simple KYC status lookup for compliance checks.

## How the code works

- The entry point in `cmd/api-gateway/main.go` loads environment configuration and starts the HTTP server.
- The app wiring in `internal/app/app.go` builds providers/validators based on `AUTH_MODE` (`mock`, `keycloak`, or `hybrid`).
- Route registration in `internal/http/router/router.go` exposes health, auth, wallet binding, and compliance endpoints.
- Auth middleware in `internal/http/middleware/auth.go` validates bearer tokens and injects token claims into the request context.
- Auth and compliance handlers orchestrate login, wallet binding, and KYC queries.

Detailed architecture and runtime flows are documented in `architecture-and-flows.md`.

## Run locally

1. Copy `.env.example` to `.env` and adjust variables as needed.
2. Start the local dependencies in `deploy/local` when available (Keycloak + Postgres).
3. Run the gateway:

```bash
go run ./cmd/api-gateway
```

## Authentication modes

- `AUTH_MODE=mock`: login and JWT validation are handled by the mock adapter.
- `AUTH_MODE=keycloak`: login and token validation are handled by Keycloak APIs.
- `AUTH_MODE=hybrid`: tries Keycloak first and falls back to mock providers/validators.

## Current endpoints

- `GET /healthz`
- `POST /auth/login`
- `POST /auth/wallet/bind` (requires `Authorization: Bearer <token>`)
- `GET /compliance/kyc/status/:subject`

## API documentation (Swagger/OpenAPI)

- OpenAPI YAML: `GET /openapi.yaml`
- Swagger UI: `GET /swagger`

The specification file is versioned in `docs/openapi.yaml` and documents only the endpoints currently implemented in this service.

To open Swagger UI locally after starting the service, access:

- `http://localhost:8080/swagger`

To get the OpenAPI document in JSON format:

- Fetch YAML from the service:
  - `curl -s http://localhost:8080/openapi.yaml > openapi.yaml`
- Convert to JSON (with `yq` v4 installed):
  - `yq -o=json '.' openapi.yaml > openapi.json`

## Main integration variables

- `KEYCLOAK_TOKEN_URL`
- `KEYCLOAK_INTROSPECTION_URL`
- `KEYCLOAK_CLIENT_ID`
- `KEYCLOAK_CLIENT_SECRET`
- `POSTGRES_DSN`

