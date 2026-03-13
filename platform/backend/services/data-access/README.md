# data-access service

gRPC microservice to centralize participants data access for backend services.

## Goal

- Expose a small data contract for participants:
  - `UpsertParticipant`
  - `GetParticipantByUser`
- Provide memory fallback for local development.
- Use Postgres with GORM when `POSTGRES_DSN` is configured.

## Structure

- `internal/grpc/contract`: gRPC JSON contract types.
- `internal/grpc/server`: gRPC handlers.
- `internal/repository`: participants repository (memory + gorm postgres).
- `cmd/data-access`: service bootstrap.

## Suggested variables

- `DATA_ACCESS_GRPC_PORT=9092`
- `DATA_ACCESS_REQUEST_TIMEOUT_SEC=5`
- `POSTGRES_DSN=<optional>`
- `DATA_ACCESS_DB_MAX_OPEN_CONNS=10`
- `DATA_ACCESS_DB_MAX_IDLE_CONNS=5`
- `DATA_ACCESS_DB_CONN_MAX_LIFETIME_SEC=300`

If `POSTGRES_DSN` is empty, the service uses in-memory repository.

