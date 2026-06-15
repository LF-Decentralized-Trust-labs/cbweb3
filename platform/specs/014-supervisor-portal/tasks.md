# Tasks: Supervisor Portal (R2-CR-8)

**Branch**: `014-supervisor-portal`
**Input**: `specs/014-supervisor-portal/` — plan.md, spec.md, data-model.md, contracts/supervisor-api.md, research.md
**Tests**: Included — Constitution Principle V mandates test-first; SC-SUP-007 requires ≥80% Go coverage; SC-SUP-008 requires Playwright E2E.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files/scenarios, no blocking dependency)
- **[US1]**: Audit Log Review · **[US2]**: ZK Pointer Verification (Scenario B) · **[US3]**: Investigation Module · **[US4]**: KYC Lookup (Scenario A)
- Scenario labels: **(A)** = scenario-a only · **(B)** = scenario-b only · **(AB)** = both (independent tasks)

---

## Phase 1: Setup — Role Middleware (Blocking)

**Purpose**: Add `RequireSupervisorRole()` to both api-gateways. Blocks all backend phases.

- [X] T001 [P] Add `RequireSupervisorRole()` convenience wrapper to `scenario-b/backend/services/api-gateway/internal/http/middleware/require_role.go` — one-liner calling `RequireRole("ROLE_SUPERVISOR")`
- [X] T002 [P] Add `RequireSupervisorRole()` convenience wrapper to `scenario-a/backend/services/api-gateway/internal/http/middleware/require_role.go` — same pattern as T001

**Checkpoint**: Middleware exists in both scenarios. All backend phases can begin.

---

## Phase 2: Foundational — Frontend HTTP Client + Auth Wiring

**Purpose**: Base `apiFetch` client and auth replacement. Blocks all frontend phases.

**⚠️ CRITICAL**: Must complete before any frontend user story phase.

- [X] T003 [P] Create `scenario-b/frontend/apps/supervisor/src/services/api/apiClient.ts` — `apiFetch<T>` with `credentials: "include"`, `X-Correlation-Id` header, typed `ApiError` on non-2xx
- [X] T004 [P] Create `scenario-a/frontend/apps/supervisor/src/services/api/apiClient.ts` — identical content to T003
- [X] T005 Replace `scenario-b/frontend/apps/supervisor/src/services/api/auth.api.ts` mockDb with real calls: `POST /api/v1/auth/login`, `GET /api/v1/auth/me`, `POST /api/v1/auth/logout` using `apiFetch`
- [X] T006 Replace `scenario-a/frontend/apps/supervisor/src/services/api/auth.api.ts` mockDb with real calls — same endpoints as T005

**Checkpoint**: Frontend can authenticate against a live backend. All frontend story phases can begin.

---

## Phase 3: User Story 1 — Audit Log Review (Priority: P1) 🎯 MVP

**Goal**: `GET /api/v1/compliance/audit/logs` live in both scenarios; `AuditVaultPage` shows real data with filters and pagination.

**Independent Test**: Supervisor logs in → navigates to Audit Vault → applies `severity=CRITICAL` filter → sees real entries from the backend. Page renders zero mock data. GOVERNANCE token gets 403.

### Tests — Write FIRST, verify they FAIL before T013

- [X] T007 [P] [US1] Write `scenario-b/backend/services/api-gateway/internal/http/handlers/supervisor_handler_test.go` — `TestSupervisorHandler_GetAuditLogs_RequiresSupervisorRole` (expect 403) and `TestSupervisorHandler_GetAuditLogs_Success` (expect 200 + log array). Must FAIL before T013.
- [X] T008 [P] [US1] Write `scenario-a/backend/services/api-gateway/internal/http/handlers/supervisor_handler_test.go` — same two tests. Must FAIL before T015.

### Backend Implementation (Scenario B)

- [X] T009 [US1] Create `scenario-b/backend/services/api-gateway/internal/http/handlers/supervisor_handler.go` — define `ComplianceAuditLister` interface (single method `GetAuditLogs`) + `SupervisorHandler` struct with that field
- [X] T010 [US1] Add `GetAuditLogs` handler method to `supervisor_handler.go` (Scenario B) — parse `category`, `severity`, `from_date`, `to_date`, `page` (default 1), `limit` (default 50, max 100) query params; call `h.lister.GetAuditLogs`; return `{ logs, page, limit, total }`
- [X] T011 [US1] Add `SupervisorHandler *handlers.SupervisorHandler` field to `Dependencies` struct in `scenario-b/backend/services/api-gateway/internal/http/router/router.go`
- [X] T012 [US1] Register route in Scenario B `router.go`: `supervisorGroup := app.Group("/api/v1/compliance", RequireCookieAuth, RequireSupervisorRole)` → `supervisorGroup.Get("/audit/logs", deps.SupervisorHandler.GetAuditLogs)`
- [X] T013 [US1] Wire `SupervisorHandler` in `scenario-b/backend/services/api-gateway/internal/app/app.go` — inject existing `complianceAdapter` as `ComplianceAuditLister`; add to `Dependencies`

### Backend Implementation (Scenario A)

- [X] T014 [P] [US1] Create `scenario-a/backend/services/api-gateway/internal/http/handlers/supervisor_handler.go` — same struct/interface shape as T009 (independent file, no cross-scenario import)
- [X] T015 [P] [US1] Add `GetAuditLogs` handler method to `supervisor_handler.go` (Scenario A) — identical logic to T010
- [X] T016 [US1] Add `SupervisorHandler` to `Dependencies` struct in `scenario-a/backend/services/api-gateway/internal/http/router/router.go`
- [X] T017 [US1] Register `GET /api/v1/compliance/audit/logs` route in Scenario A `router.go` — same guard pattern as T012
- [X] T018 [US1] Wire `SupervisorHandler` in `scenario-a/backend/services/api-gateway/internal/app/app.go`

### Frontend Implementation (both scenarios, parallel)

- [X] T019 [P] [US1] Extend `scenario-b/frontend/apps/supervisor/src/types/audit.types.ts` — add `severity?: "INFO"|"WARNING"|"CRITICAL"`, `category?: "GOVERNANCE"|"COMPLIANCE"|"SETTLEMENT"` to `AuditLogEntry`; add `AuditLogFilters` interface (see data-model.md)
- [X] T020 [P] [US1] Extend `scenario-a/frontend/apps/supervisor/src/types/audit.types.ts` — identical additions to T019
- [X] T021 [US1] Replace `scenario-b/frontend/apps/supervisor/src/services/api/audit.api.ts` mockDb — call `apiFetch<{logs: AuditLogEntry[], page: number, limit: number, total: number}>("/api/v1/compliance/audit/logs", ...)` accepting `AuditLogFilters` params
- [X] T022 [US1] Replace `scenario-a/frontend/apps/supervisor/src/services/api/audit.api.ts` mockDb — identical to T021
- [X] T023 [US1] Update `scenario-b/frontend/apps/supervisor/src/pages/AuditVaultPage.tsx` — add filter form (category/severity dropdowns, from/to date inputs), pagination controls (prev/next + page indicator), remove decrypt-transaction panel (moves to InvestigationPage in Phase 5)
- [X] T024 [US1] Update `scenario-a/frontend/apps/supervisor/src/pages/AuditVaultPage.tsx` — identical changes to T023

### E2E Tests

- [X] T025 [P] [US1] Create `scenario-b/tests/supervisor/audit-vault.spec.ts` — Playwright: happy path (supervisor token → `/audit` → real logs rendered) + 403 case (governance token → "Insufficient privileges" shown)
- [X] T026 [P] [US1] Create `scenario-a/tests/supervisor/audit-vault.spec.ts` — identical test structure to T025

**Checkpoint**: User Story 1 fully functional. MVP deliverable: supervisors in both scenarios can read real audit logs with filters. Run `go test ./internal/http/handlers/... -run TestSupervisorHandler` (both scenarios) and Playwright `audit-vault.spec.ts`.

---

## Phase 4: User Story 2 — ZK Pointer Compliance Verification (Priority: P2, Scenario B only)

**Goal**: `GET /api/v1/compliance/zk-pointer/verify` live; `ZKPointerPanel` renders real state from the Trust Registry.

**Independent Test**: Supervisor enters known `bank_id` + `commitment_hash` → panel shows `VALID` / `INVALID` / `EXPIRED` state from backend. No mocks.

### Tests — Write FIRST, verify they FAIL before T030

- [X] T027 [US2] Add to `scenario-b/.../supervisor_handler_test.go`: `TestSupervisorHandler_VerifyZKPointer_Valid`, `_NotFound` (404), `_Expired`, `_MissingParams` (400). Must FAIL before T030.

### Backend Implementation

- [X] T028 [US2] Add `GetZKPointerRecord(ctx context.Context, bankID, commitmentHash string) (*zkPointerRecord, error)` method to `scenario-b/backend/services/compliance/internal/services/zk_compliance_gate.go` — returns pointer_id and expires_at for building HTTP response
- [X] T029 [US2] Instantiate `ZKComplianceGate` in `scenario-b/backend/services/api-gateway/internal/app/app.go` from the compliance `db` connection (`services.NewZKComplianceGate(db)`) and pass to `SupervisorHandler` constructor — **this gate is currently nil at runtime** (see research D-02)
- [X] T030 [US2] Add `ZKPointerVerifier` interface + `VerifyZKPointer` handler method to `scenario-b/.../supervisor_handler.go` — validate `bank_id` and `commitment_hash` query params; call gate; map error to INVALID/EXPIRED; return `ZKPointerVerification` JSON (see contracts/supervisor-api.md)
- [X] T031 [US2] Register `GET /api/v1/compliance/zk-pointer/verify` in Scenario B `router.go` under existing `supervisorGroup`

### Frontend Implementation

- [X] T032 [P] [US2] Create `scenario-b/frontend/apps/supervisor/src/types/zk-pointer.types.ts` — `ZKPointerState`, `ZKPointerVerification` (see data-model.md)
- [X] T033 [P] [US2] Create `scenario-b/frontend/apps/supervisor/src/services/api/zk-pointer.api.ts` — `verifyZKPointer(bankId, commitmentHash)` → `apiFetch` to `GET /api/v1/compliance/zk-pointer/verify`
- [X] T034 [US2] Create `scenario-b/frontend/apps/supervisor/src/components/supervisor/ZKPointerPanel.tsx` — form with `bank_id` + `commitment_hash` inputs; result card with state badge (VALID=success, INVALID/EXPIRED=destructive), `pointer_id`, `expires_at`
- [X] T035 [US2] Add `ZKPointerPanel` to `scenario-b/frontend/apps/supervisor/src/pages/AuditVaultPage.tsx` as a collapsible card section titled "ZK Pointer Verification"

**Checkpoint**: Scenario B supervisor can verify ZK-pointer authenticity. Run `go test -run TestSupervisorHandler_VerifyZKPointer`.

---

## Phase 5: User Story 3 — Investigation Module / Deanonymisation (Priority: P2)

**Goal**: Supervisor can open, co-sign, and track AML/CFT disclosure requests. Scenario B calls existing `/api/v2/oversight/*`; Scenario A gets a full new backend.

**Independent Test**: Two supervisors in separate sessions open and co-sign a disclosure request → state reaches `QUORUM_REACHED`. Works in both scenarios.

### Tests — Write FIRST, verify they FAIL before T043 (Scenario A only)

- [X] T036 [US3] Write `scenario-a/backend/services/compliance/internal/services/oversight_service_test.go` — `TestOversightService_OpenDisclosure`, `_SignDisclosure_QuorumReached`, `_SignDisclosure_AlreadySigned`, `_SignDisclosure_Expired`, `_GetDisclosureStatus_AutoExpire`. Must FAIL before T042.
- [X] T037 [US3] Write `scenario-a/backend/services/api-gateway/internal/http/handlers/oversight_handler_test.go` — `TestOversightHandler_OpenDisclosure_Success`, `_MissingFields` (400), `_InvalidReasonCode` (400); `TestOversightHandler_SignDisclosure_AlreadySigned` (400); `TestOversightHandler_GetDisclosureStatus_NotFound` (404). Must FAIL before T045.

### Backend — Scenario A (new build)

- [X] T038 [US3] Create `scenario-a/backend/services/compliance/internal/domain/disclosure.go` — `DisclosureRequest` + `DisclosureSignature` GORM structs; `DisclosurePending`, `DisclosureQuorumReached`, `DisclosureExpired` state constants (independent reimplementation, no scenario-b import)
- [X] T039 [US3] Extend `scenario-a/backend/services/compliance/internal/db/init/migrate.go` — add `AutoMigrate(&domain.DisclosureRequest{}, &domain.DisclosureSignature{})` (idempotent)
- [X] T040 [US3] Create `scenario-a/backend/services/compliance/internal/services/oversight_service.go` — `OversightService{db}` with `OpenDisclosure` (72h expiry, reason_code stored), `SignDisclosure` (unique-signer guard, quorum counter, QUORUM_REACHED transition), `GetDisclosureStatus` (auto-expire on read)
- [X] T041 [US3] Create `scenario-a/backend/services/api-gateway/internal/http/handlers/oversight_handler.go` — `OversightHandler` with `OpenDisclosure` (validate reason_code enum → 400 if invalid), `SignDisclosure`, `GetDisclosureStatus`; append lifecycle events to audit_log
- [X] T042 [US3] Add `OversightHandler *handlers.OversightHandler` to Scenario A `Dependencies` struct in `router.go`
- [X] T043 [US3] Register `/api/v1/oversight/*` routes in Scenario A `router.go` with `RequireSupervisorRole` guard on POST endpoints; `GetDisclosureStatus` is open (no role guard, matches Scenario B behaviour)
- [X] T044 [US3] Wire `OversightHandler` in `scenario-a/backend/services/api-gateway/internal/app/app.go` — instantiate `OversightService(db)` → `OversightHandler(svc)` → inject into `Dependencies`

### Frontend — Both Scenarios (parallel per scenario)

- [X] T045 [P] [US3] Create `scenario-b/frontend/apps/supervisor/src/types/investigation.types.ts` — `ReasonCode`, `DisclosureState`, `DisclosureRequest`, `OpenDisclosurePayload`, `SignDisclosurePayload` (see data-model.md)
- [X] T046 [P] [US3] Create `scenario-a/frontend/apps/supervisor/src/types/investigation.types.ts` — identical content to T045
- [X] T047 [P] [US3] Create `scenario-b/frontend/apps/supervisor/src/services/api/oversight.api.ts` — `openDisclosure`, `signDisclosure`, `getDisclosureStatus` calling `/api/v2/oversight/*` via `apiFetch`
- [X] T048 [P] [US3] Create `scenario-a/frontend/apps/supervisor/src/services/api/oversight.api.ts` — same methods calling `/api/v1/oversight/*`
- [X] T049 [US3] Create `scenario-b/frontend/apps/supervisor/src/pages/InvestigationPage.tsx` — Section 1: Open Request form (`tx_ref`, `requestor_id`, `reason_code` enum select); Section 2: Sign Request form; Section 3: Status tracker with 10s polling while `PENDING`, stops on `QUORUM_REACHED`/`EXPIRED`, shows "Proceed to Paladin disclosure" callout on quorum
- [X] T050 [US3] Create `scenario-a/frontend/apps/supervisor/src/pages/InvestigationPage.tsx` — identical structure to T049
- [X] T051 [P] [US3] Add `/investigation` route to `scenario-b/frontend/apps/supervisor/src/routes/index.tsx`
- [X] T052 [P] [US3] Add `/investigation` route to `scenario-a/frontend/apps/supervisor/src/routes/index.tsx`
- [X] T053 [P] [US3] Add Investigation sidebar nav item to `scenario-b/frontend/apps/supervisor/src/components/layout/Sidebar.tsx`
- [X] T054 [P] [US3] Add Investigation sidebar nav item to `scenario-a/frontend/apps/supervisor/src/components/layout/Sidebar.tsx`

**Checkpoint**: Full investigation flow works end-to-end in both scenarios. Run `go test ./internal/services/... -run TestOversightService` + `./internal/http/handlers/... -run TestOversightHandler` (Scenario A). Test two-supervisor co-sign flow manually per quickstart.md step 4–6.

---

## Phase 6: User Story 4 — KYC Status Lookup (Priority: P3, Scenario A only)

**Goal**: Scenario A compliance verification panel lets supervisors look up KYC/credential status by subject ID. Reuses existing `GET /api/v1/compliance/kyc/status/:subject` — no new backend needed.

**Independent Test**: Scenario A supervisor enters a subject ID → panel shows `ACTIVE` / `FROZEN` / `REVOKED` from the real `ComplianceHandler.GetKYCStatus` endpoint.

- [X] T055 [US4] Create `scenario-a/frontend/apps/supervisor/src/components/supervisor/KYCStatusPanel.tsx` — form with `subject` input; on submit calls `GET /api/v1/compliance/kyc/status/:subject` via `apiFetch`; displays `status` badge (destructive if FROZEN/REVOKED), `last_updated_at`, freeze reason if present
- [X] T056 [US4] Add `KYCStatusPanel` to `scenario-a/frontend/apps/supervisor/src/pages/AuditVaultPage.tsx` as a collapsible card section titled "Credential Verification"

**Checkpoint**: Scenario A supervisor can look up participant credential status. No mock data. Test with known subject IDs from quickstart.md.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Replace remaining mockDb calls; coverage sweep; documentation.

- [X] T057 [P] Replace `scenario-b/frontend/apps/supervisor/src/services/api/governance.api.ts` mockDb — call `GET /api/v1/compliance/participants` via `apiFetch`
- [X] T058 [P] Replace `scenario-a/frontend/apps/supervisor/src/services/api/governance.api.ts` mockDb — same endpoint as T057
- [X] T059 [P] Replace `scenario-b/frontend/apps/supervisor/src/services/api/network.api.ts` mockDb — call appropriate overview endpoint (verify exact route in router during implementation)
- [X] T060 [P] Replace `scenario-a/frontend/apps/supervisor/src/services/api/network.api.ts` mockDb — same as T059
- [X] T061 [P] Replace `scenario-b/frontend/apps/supervisor/src/services/api/stability.api.ts` mockDb — call pool status or governance parameters endpoint
- [X] T062 [P] Replace `scenario-a/frontend/apps/supervisor/src/services/api/stability.api.ts` mockDb — same as T061
- [X] T063 Remove `scenario-b/frontend/apps/supervisor/src/services/mocks/mock-db.ts` after confirming zero remaining imports — replaced mockDb.decryptTransaction with apiFetch to /api/v1/compliance/decrypt-transaction (degrades gracefully until backend endpoint is implemented)
- [X] T064 Remove `scenario-a/frontend/apps/supervisor/src/services/mocks/mock-db.ts` — same approach as T063
- [X] T065 Run `go test -cover ./internal/http/handlers/... ./internal/services/...` in both scenarios on new files; add missing test cases for any file below 80% branch coverage
- [X] T066 Add specific test to `scenario-b/supervisor_handler_test.go`: `TestSupervisorHandler_GetAuditLogs_ForbiddenForGovernanceRole` — assert 403 when caller has `ROLE_GOVERNANCE` (SC-SUP-006)
- [X] T067 Add same 403-for-governance test to `scenario-a/supervisor_handler_test.go`
- [X] T068 [P] Update `scenario-a/README.md` — set Supervisor Portal feature status to "In progress"
- [X] T069 [P] Update `scenario-b/README.md` — set Supervisor Portal feature status to "In progress"

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (T001–T002) ──────────────────────────────────────► all backend phases
Phase 2 (T003–T006) ──────────────────────────────────────► all frontend phases
Phase 3 backend (T007–T018) ──► Phase 3 frontend (T019–T026)
Phase 1 + Phase 3 backend ────► Phase 4 (T027–T035, Scenario B)
Phase 1 ──────────────────────► Phase 5 backend A (T036–T044)
Phase 2 + Phase 5 backend A ──► Phase 5 frontend (T045–T054)
Phase 2 ──────────────────────► Phase 6 (T055–T056, no new backend)
Phases 3–6 complete ──────────► Phase 7 (T057–T069)
```

### User Story Dependencies

- **US1 (P1)**: Depends on Phase 1 (T001–T002) + Phase 2 (T003–T006). No story dependencies.
- **US2 (P2)**: Depends on Phase 1 + US1 backend (T013 — SupervisorHandler exists). No frontend dependency on US1.
- **US3 (P2)**: Depends on Phase 1 (backend A) + Phase 2 (frontend). No dependency on US1 or US2.
- **US4 (P3)**: Depends on Phase 2 only. Fully independent.

### Within Each User Story

- Tests MUST be written and confirmed FAILING before implementation begins
- Models → Services → Handlers → Routes → Frontend
- Story complete and tested before moving to next priority

### Parallel Opportunities

- **T001 || T002**: RequireSupervisorRole in both scenarios
- **T003 || T004**: apiClient.ts in both scenarios
- **T005 || T006**: auth.api.ts replacement
- **T007 || T008**: supervisor_handler tests
- **T009 || T014**: SupervisorHandler struct creation (different files)
- **T019 || T020**: audit.types.ts extension
- **T025 || T026**: Playwright specs
- **T032 || T033**: zk-pointer types + API service
- **T045 || T046**, **T047 || T048**, **T051–T054**: investigation types, services, routes
- **T057–T062**: remaining mockDb replacements
- **T068 || T069**: README updates

---

## Parallel Example: User Story 1 Backend

```bash
# After Phase 1 completes — launch in parallel:
Agent A: T007 + T009 + T010 + T011 + T012 + T013  # Scenario B backend
Agent B: T008 + T014 + T015 + T016 + T017 + T018  # Scenario A backend

# After Phase 2 completes — launch in parallel:
Agent C: T019 + T021 + T023  # Scenario B frontend
Agent D: T020 + T022 + T024  # Scenario A frontend

# Both agents can run E2E tests in parallel:
Agent C: T025
Agent D: T026
```

---

## Implementation Strategy

### MVP (User Story 1 — Audit Vault only)

1. ✅ Phase 1: T001–T002 (middleware, ~15 min)
2. ✅ Phase 2: T003–T006 (apiClient + auth, ~30 min)
3. ✅ Phase 3: T007–T026 (backend + frontend + E2E for both scenarios, ~3–4h)
4. **STOP and VALIDATE**: Run Playwright `audit-vault.spec.ts` in both scenarios
5. **Demo**: Supervisor reads real audit logs with filters

### Full Delivery

| Story | Phase | Est. Tasks | Parallel? |
|---|---|---|---|
| US1 Audit Vault | 3 | 20 tasks | A/B parallel |
| US2 ZK Pointer | 4 | 9 tasks | B only |
| US3 Investigation | 5 | 19 tasks | A/B parallel after A backend |
| US4 KYC Lookup | 6 | 2 tasks | A only, quick |
| Polish | 7 | 13 tasks | Mostly parallel |

---

## Notes

- **[P]**: tasks in the same phase touching different files (no write conflict)
- `mock-db.ts` deletion (T063/T064) must come AFTER all replacements in T005–T006, T021–T022, T057–T062
- Before T029: confirm `ZKComplianceGate` can access the compliance DB from the api-gateway process (same DB connection or separate); adjust wiring if they are separate DBs
- `reason_code` validation belongs in the HTTP handler (T041), not the service (T040) — invalid values must return HTTP 400, not 422
- Scenario A's `OversightService` must not import any `scenario-b/` package — add a `grep -r "scenario-b" scenario-a/` CI check if not already present
