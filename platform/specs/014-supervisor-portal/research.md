# Research: Supervisor Portal (014)

## Decision Log

### D-01: Role enforcement middleware
**Decision**: Create `RequireSupervisorRole()` convenience function in both scenarios' middleware packages.  
**Rationale**: `RequireRole(roles ...string)` exists, so the new function is a one-liner wrapper — `RequireRole("ROLE_SUPERVISOR")`. No `RequireSupervisorRole` exists today; adding it keeps the RBAC surface consistent with the existing `RequireCentralBankRole()` / `RequireCommercialBankRole()` pattern.  
**Alternatives considered**: Inline `RequireRole("ROLE_SUPERVISOR")` call in each route — rejected because it scatters the role string literal and loses the intent-communicating name.

### D-02: Scenario B ZKComplianceGate wiring
**Decision**: Instantiate `ZKComplianceGate` from the compliance DB connection in `app.go` and inject it into `SupervisorHandler` as a named field (not via `SwapGates`).  
**Rationale**: Investigation found `SwapGates.ComplianceGate` is **nil at runtime** — only `CircuitBreakerGate` and `PoolStatusGate` are wired (app.go lines 355–365). The gate exists but is never created. For the supervisor endpoint we need an instance; we create it directly in app wiring from the same `db` reference already available in `app.go`. Adding it to `SwapGates` would be a side-effect change to swap flow; injecting it separately into `SupervisorHandler` is safer.  
**Alternatives considered**: Wire it into `SwapGates.ComplianceGate` simultaneously — deferred; that change belongs in the swap correctness track, not here.

### D-03: Audit logs route and handler placement
**Decision**: New `SupervisorHandler` in both api-gateways; the handler delegates to the **same** `GetAuditLogs` compliance adapter method already used by `GovernanceHandler`. Route: `GET /api/v1/compliance/audit/logs`.  
**Rationale**: `GovernanceHandler` already accumulates KYC, circuit-breaker, system parameters, user management, and audit log endpoints. Adding a `ROLE_SUPERVISOR`-guarded branch inside it would make role logic implicit. A dedicated handler makes the permission contract explicit.  
**Alternatives considered**: Add a second route to `GovernanceHandler` with `RequireSupervisorRole` guard — rejected because `GovernanceHandler` would then serve two distinct roles.

### D-04: Scenario A oversight service implementation
**Decision**: Independent reimplementation of `OversightService` and `DisclosureRequest` domain model in `scenario-a/backend/services/compliance/internal/`. No code shared with Scenario B.  
**Rationale**: Constitution Principle I prohibits cross-scenario sharing. The logic is identical (2-of-N quorum, 72h expiry) but must live in Scenario A's own package tree.  
**Alternatives considered**: Shared library module — prohibited by constitution without a versioned shared library release lifecycle.

### D-05: Scenario A api-gateway oversight handler route
**Decision**: `POST /api/v1/oversight/disclosure-request`, `POST /api/v1/oversight/disclosure-sign`, `GET /api/v1/oversight/disclosure-status/:requestID`. New `OversightHandler` added to Scenario A's `Dependencies` struct.  
**Rationale**: Scenario A uses `/api/v1` namespace throughout. The `Dependencies` struct currently has no `OversightService` field; it must be added alongside the handler.

### D-06: Frontend apiClient
**Decision**: Create `src/services/api/client.ts` in both supervisor apps — a thin `fetch` wrapper that:
- Reads base URL from `import.meta.env.VITE_API_BASE_URL`
- Attaches `credentials: "include"` (cookie-based session)
- Injects `X-Correlation-Id` header (random UUID per request)
- Throws a typed `ApiError` on non-2xx  
**Rationale**: All five API modules (`audit.api.ts`, `auth.api.ts`, `governance.api.ts`, `stability.api.ts`, `network.api.ts`) currently call `mockDb`. A shared client avoids duplicating fetch boilerplate and ensures the correlation-id header propagates consistently.

### D-07: Scenario A compliance gRPC GetAuditLogs
**Decision**: Use existing implementation — confirmed present in `scenario-a/backend/services/compliance/internal/grpc/server/server.go` (line 145) with contract constant at `scenario-a/backend/services/compliance/internal/grpc/contract/contract.go` (line 20). No new proto work needed.

### D-08: Investigation Module UI placement
**Decision**: The Investigation Module lives as a new `InvestigationPage.tsx` (new route `/investigation`) in both supervisor apps, rather than being embedded in `AuditVaultPage`.  
**Rationale**: `AuditVaultPage` already handles audit log display + transaction decrypt panel; adding open/sign/track disclosure forms would make it too wide in scope. A dedicated page also makes the permission model clearer (`/investigation` can show a role-guard message independently).  
**Alternatives considered**: Tab within `AuditVaultPage` — rejected because it conflates two different supervisory actions (read-only audit review vs. active investigation initiation).
