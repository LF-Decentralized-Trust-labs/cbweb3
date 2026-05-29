# apis

> [scenario-a](../README.md) › apis

The **apis** directory centralizes all API contracts for the platform: REST OpenAPI specifications, gRPC Protobuf definitions, and auto-generated client SDKs. It is the single source of truth for inter-service interfaces.

---

## Structure

```
apis/
├── openapi/               REST API specifications (OpenAPI 3.x YAML)
│   ├── auth.yaml
│   ├── compliance.yaml
│   └── payment_orchestrator.yaml
├── proto/                 gRPC service definitions (Protocol Buffers)
│   ├── auth/              Auth service RPCs and message types
│   ├── compliance/        Compliance service RPCs and message types
│   ├── payment_orchestrator/  Payment orchestrator RPCs and message types
│   └── buf.yaml           Buf code generation config
└── sdk/                   Auto-generated client libraries
    ├── java/
    ├── python/
    └── typescript/
```

---

## OpenAPI (REST)

The `openapi/` directory contains YAML specifications for the REST interfaces exposed by the [api-gateway](../backend/services/api-gateway/README.md). Each spec maps to the endpoints a given entity's gateway exposes.

The api-gateway also serves its spec at runtime:

```bash
curl http://localhost:8080/openapi.yaml
# or browse: http://localhost:8080/swagger
```

---

## Protobuf / gRPC

The `proto/` directory defines the service contracts between internal gRPC microservices. These definitions are the authoritative source for:

- [auth service](../backend/services/auth/README.md) — `proto/auth/`
- [compliance service](../backend/services/compliance/README.md) — `proto/compliance/`
- [payment-orchestrator](../backend/services/payment-orchestrator/README.md) — `proto/payment_orchestrator/`

Code generation uses [Buf](https://buf.build/):

```bash
make proto-gen       # Generate Go, TypeScript, Python, Java stubs
make proto-lint      # Lint proto files
make proto-breaking  # Check for breaking changes against main
```

Generated Go stubs are output to `backend/shared/proto/` and imported by all services.

---

## SDKs

The `sdk/` directory contains auto-generated client libraries for external consumers of the platform's REST API. Generated from the OpenAPI specs via [openapi-generator](https://github.com/OpenAPITools/openapi-generator).

| Language | Path | Status |
|----------|------|--------|
| TypeScript | `sdk/typescript/` | Generated |
| Python | `sdk/python/` | Generated |
| Java | `sdk/java/` | Generated |

---

## Related

- [api-gateway](../backend/services/api-gateway/README.md) — serves the OpenAPI spec at `/openapi.yaml`
- [auth](../backend/services/auth/README.md) — gRPC contract defined in `proto/auth/`
- [compliance](../backend/services/compliance/README.md) — gRPC contract defined in `proto/compliance/`
- [payment-orchestrator](../backend/services/payment-orchestrator/README.md) — gRPC contract defined in `proto/payment_orchestrator/`
