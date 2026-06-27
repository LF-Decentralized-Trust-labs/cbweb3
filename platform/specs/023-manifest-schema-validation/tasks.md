# Tasks: Manifest Schema and Validation (TK-1)

**Input**: Design documents from `/specs/023-manifest-schema-validation/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Included — required by Constitution Principle V (test-first, failing test before implementation).

**Organization**: Tasks grouped by user story. US1/US2/US3 share `validate.go` and `validate_test.go`; each phase adds failing tests then the implementation that makes them pass.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks in this phase)
- **[Story]**: Maps to user story from spec.md (US1–US4)

---

## Phase 1: Setup

**Purpose**: Add the only new dependency; no source files yet.

- [x] T001 Add `gopkg.in/yaml.v3` to `scenario-a/toolkit/go.mod` via `go get gopkg.in/yaml.v3` from `scenario-a/toolkit/`

**Checkpoint**: `go mod tidy` runs clean; `go.sum` updated.

---

## Phase 2: Foundational (Blocking Prerequisite)

**Purpose**: Struct definitions that all test and implementation files import. Must exist before any test file compiles.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Create `scenario-a/toolkit/engine/manifest/types.go` with all struct definitions from `data-model.md`: `Manifest`, `Metadata`, `Spec`, `Spoke`, `Node`, `Port`, `Relay` — YAML tags on every field; package-level doc comment; Apache-2.0 header

**Checkpoint**: `go build ./engine/manifest/...` compiles (no functions yet, just types).

---

## Phase 3: User Story 1 — Valid manifest accepted (Priority: P1) 🎯 MVP

**Goal**: `Load` parses a YAML file into `*Manifest`; `Validate` returns nil for a correctly-filled manifest.

**Independent Test**: Run `go test ./engine/manifest/... -run TestLoad_ValidManifest -run TestValidate_ValidManifest` — both pass; the minimal manifest from `contracts/manifest-yaml-format.md` is the fixture.

- [x] T003 [US1] Write failing tests `TestLoad_ValidManifest` (file → struct roundtrip) and `TestValidate_ValidManifest` (valid manifest → nil error, with and without optional fields) in `scenario-a/toolkit/engine/manifest/validate_test.go` — tests must fail to compile until T004
- [x] T004 [US1] Create `scenario-a/toolkit/engine/manifest/validate.go` with function stubs `Load(path string) (*Manifest, error)` and `Validate(m *Manifest) error` — both return `errors.New("not implemented")`; tests now compile and fail at runtime
- [x] T005 [US1] Implement `Load` (read file → `yaml.Unmarshal` into `*Manifest`, return error on missing file / bad YAML) and `Validate` happy path (returns nil for valid input) in `scenario-a/toolkit/engine/manifest/validate.go`

**Checkpoint**: `go test ./engine/manifest/... -run TestLoad -run TestValidate_ValidManifest` — all pass. US1 acceptance scenarios verified.

---

## Phase 4: User Story 2 — Missing required field surfaces a clear error (Priority: P1)

**Goal**: `Validate` returns a non-nil error for every missing required field; each error names the exact field path; all failures reported in one call.

**Independent Test**: Run `go test ./engine/manifest/... -run TestValidate_MissingRequiredFields` — table cases for each of the 12 required fields pass; multi-field case returns one error per missing field.

- [x] T006 [US2] Add table-driven test `TestValidate_MissingRequiredFields` to `scenario-a/toolkit/engine/manifest/validate_test.go`; cases: one case per required field (`apiVersion`, `kind`, `metadata.name`, `spec.scenario`, `spec.role`, `spec.mode`, `spec.spoke.id`, `spec.spoke.chainId`, `spec.spoke.currency`, `spec.node.advertisedHost`, `spec.image`, `spec.keyProvider`, `spec.certSource`); one multi-field case verifying all errors reported in a single call; `advertisedHost` empty-string case; assert each error string contains the field path — tests fail (Validate returns nil)
- [x] T007 [US2] Implement required-field checks in `Validate` in `scenario-a/toolkit/engine/manifest/validate.go`: collect errors into `[]error`, use `errors.Join` to return all at once; `advertisedHost` check must include the phrase "never inferred" in the error message

**Checkpoint**: `go test ./engine/manifest/...` — Phase 3 tests still pass; Phase 4 tests now pass.

---

## Phase 5: User Story 3 — Invalid enum value surfaces a clear error (Priority: P1)

**Goal**: `Validate` returns a descriptive error for enum violations, naming the field, the invalid value, and the accepted values.

**Independent Test**: Run `go test ./engine/manifest/... -run TestValidate_InvalidEnumValues` — all cases pass; error messages include field path, bad value, and accepted-values list.

- [x] T008 [US3] Add table-driven test `TestValidate_InvalidEnumValues` to `scenario-a/toolkit/engine/manifest/validate_test.go`; cases: `spec.role` invalid (e.g. `"central"`), `spec.mode` invalid (e.g. `"init"`), `spec.scenario: "b"`, `spec.environment: "dev"` (invalid when present); assert each error string contains the field path, the invalid value, and the accepted values — tests fail
- [x] T009 [US3] Implement enum constraint checks for `spec.role`, `spec.mode`, `spec.scenario`, `spec.environment` in `Validate` in `scenario-a/toolkit/engine/manifest/validate.go`; error format: `"<field>: invalid value \"<v>\"; accepted values are: <a>, <b>"`; integrate with existing `errors.Join` collector

**Checkpoint**: `go test ./engine/manifest/...` — all P1 tests pass (Phases 3, 4, 5).

---

## Phase 6: User Story 4 — IDE autocompletion and inline error highlighting (Priority: P2)

**Goal**: JSON Schema draft-07 file in `provisioning/schema/v1/` that YAML Language Server uses for editor-time validation and autocompletion, without any plugin configuration beyond a `$schema` header.

**Independent Test**: Open a new manifest YAML with `# yaml-language-server: $schema: ../../provisioning/schema/v1/participant-deployment.schema.yaml` in VS Code; omit `spec.node.advertisedHost` → editor shows inline error; type `role: ` → editor suggests `central-bank`, `commercial-bank`.

- [x] T010 [P] [US4] Create `scenario-a/provisioning/schema/v1/participant-deployment.schema.yaml` as JSON Schema draft-07: `$schema` header, `title`, `required` arrays at each level, `enum` for `role`/`mode`/`scenario`/`environment`, `properties` with `description` on every field (special note on `advertisedHost`: "Required. Must be the externally reachable address of the node. Never inferred from co-location or container IP."), `additionalProperties: false` at top level, use `definitions` (not `$defs`) for reusable sub-schemas (`Port`, `Spoke`, `Node`, `Relay`)

**Checkpoint**: Schema file opens in VS Code without parse errors; `$schema` header on a test manifest enables autocompletion and inline field errors.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and documentation consistency.

- [x] T011 [P] Run `go test -v ./engine/manifest/...` from `scenario-a/toolkit/` and confirm all tests pass with descriptive names matching the user stories
- [x] T012 Verify `scenario-a/toolkit/go.mod` and `go.sum` are committed (no dirty `go mod tidy` changes)

**Checkpoint**: `go test ./...` clean from `scenario-a/toolkit/`. Schema file present and parseable.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (go.mod must have yaml.v3 before types.go compiles)
- **User Stories (Phases 3–6)**: All depend on Phase 2 (types.go must exist)
  - Phase 3 → Phase 4 → Phase 5 (sequential: each adds to the same validate.go)
  - Phase 6 is independent of Phases 3–5 (different file entirely)
- **Polish (Phase 7)**: Depends on all prior phases

### User Story Dependencies

- **US1 (Phase 3)**: Start after Phase 2 — no story dependencies
- **US2 (Phase 4)**: Start after Phase 3 — builds on the `Validate` stub from T005
- **US3 (Phase 5)**: Start after Phase 4 — extends the same `Validate` function
- **US4 (Phase 6)**: Start after Phase 2 — independent of US1/US2/US3 (schema file only)

### Within Each User Story

1. Write failing test(s) — MUST FAIL before implementation
2. Implement to make tests pass
3. Run full test suite — prior tests must not regress

### Parallel Opportunities

- T010 (US4 schema file) can run in parallel with any of Phases 3–5 since it touches a different file
- T011 and T012 in Phase 7 can run in parallel

---

## Parallel Example: US4 alongside US2

```bash
# Thread 1 (after Phase 2):
T006 → T007  (US2: missing-field validation)

# Thread 2 (after Phase 2, parallel):
T010          (US4: JSON Schema file — no code dependency)
```

---

## Implementation Strategy

### MVP First (US1 only — phases 1–3)

1. Phase 1: Add yaml.v3 dependency
2. Phase 2: Create types.go
3. Phase 3: Write tests → stubs → implement Load + Validate happy path
4. **STOP**: `go test -run TestLoad -run TestValidate_ValidManifest` passes
5. Engine can now call `Load` + `Validate` on a valid manifest

### Incremental Delivery

1. Phases 1–3 → Load + happy-path Validate ← engine can read manifests
2. Phase 4 → missing-field errors ← operators get actionable errors
3. Phase 5 → enum errors ← operators see all constraint violations
4. Phase 6 → JSON Schema ← editor tooling activated

### Solo Developer Sequence

T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010 → T011 → T012

---

## Notes

- `validate_test.go` is the single test file; all test functions live there
- `validate.go` is the single implementation file; `Load` and `Validate` both live there
- `types.go` is read-only after T002 — do not add logic to it
- The `advertisedHost` error message must contain "never inferred" to satisfy the spec wording (US2 scenario 1)
- Enum error format is fixed: `"<field>: invalid value \"<v>\"; accepted values are: <a>, <b>"` — tests assert on this exact shape
- The JSON Schema file (T010) is never imported by Go code; it is for editors and CI only

---

## Phase 8: Convergence

*Generated by `/speckit-converge` — 2026-06-27. Appended per convergence contract; existing tasks unchanged.*

- [x] T013 Update error message for `spec.scenario: "b"` in `scenario-a/toolkit/engine/manifest/validate.go` to include "Scenario B is out of scope for this toolkit", and add `wantInErrs: []string{"out of scope"}` assertion to the `invalid spec.scenario` case in `scenario-a/toolkit/engine/manifest/validate_test.go` per US3/AC3 (partial)
