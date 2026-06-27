# Feature Specification: Manifest Schema and Validation for Participant Deployment

**Feature Branch**: `023-manifest-schema-validation`
**Created**: 2026-06-27
**Status**: Draft
**Input**: User description: "TK-1 — Criar schema do manifesto YAML com validação JSON Schema draft-07 para ParticipantDeployment"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Valid manifest accepted without errors (Priority: P1)

An operator preparing to provision a participant has a correctly-filled YAML manifest. When they run the provisioning toolkit's validate command, the tool accepts the manifest without errors and proceeds.

**Why this priority**: Core path — every other provisioning action depends on manifest validation passing. This is what "works" means.

**Independent Test**: Provide the minimal valid manifest example (central-bank, mode: found, local profile) to the validator and confirm it returns without errors.

**Acceptance Scenarios**:

1. **Given** a manifest file with all required fields correctly filled, **When** the operator runs the validate command, **Then** the tool returns success with no error messages.
2. **Given** a valid manifest with optional fields omitted, **When** the operator runs the validate command, **Then** the tool accepts it (optional fields are not required).

---

### User Story 2 - Missing required field surfaces a clear, actionable error (Priority: P1)

An operator has a manifest file that is missing a required field (e.g., forgot `advertisedHost`). When they run the provisioning toolkit, the tool immediately stops and prints a specific, actionable error message identifying the exact missing field by its full path.

**Why this priority**: The "fail fast with clear errors" requirement. The operator must know exactly what to fix without guessing or reading source code.

**Independent Test**: Provide a manifest with `spec.node.advertisedHost` removed to the validator and confirm the error message names that exact field path.

**Acceptance Scenarios**:

1. **Given** a manifest missing `spec.node.advertisedHost`, **When** the operator runs validate, **Then** the tool prints an error naming `spec.node.advertisedHost` as missing and states it must be explicitly set.
2. **Given** a manifest missing `spec.role`, **When** the operator runs validate, **Then** the tool prints an error naming `spec.role`.
3. **Given** a manifest missing multiple required fields, **When** the operator runs validate, **Then** the tool reports all missing fields in a single pass.

---

### User Story 3 - Invalid field value surfaces a clear error with accepted values (Priority: P1)

An operator provides a manifest with an invalid value for a constrained field — for example, `role: central` instead of `role: central-bank`. The tool immediately stops and reports the invalid value together with the list of accepted values.

**Why this priority**: Enum constraints prevent silent misconfiguration that would only fail later during deployment, after significant time is spent.

**Independent Test**: Provide a manifest with `role: central` to the validator and confirm the error message names the field, the bad value, and the accepted values.

**Acceptance Scenarios**:

1. **Given** a manifest with `spec.role: central` (invalid), **When** the operator runs validate, **Then** the error names the field, the invalid value, and the accepted values (`central-bank`, `commercial-bank`).
2. **Given** a manifest with `spec.mode: init` (invalid), **When** the operator runs validate, **Then** the error names `spec.mode` and lists accepted values (`found`, `join`).
3. **Given** a manifest with `spec.scenario: b`, **When** the operator runs validate, **Then** the error rejects it with a message stating Scenario B is out of scope.

---

### User Story 4 - IDE autocompletion and inline error highlighting (Priority: P2)

A developer authoring a new manifest in their editor (VS Code with YAML extension) sees autocompletion suggestions for constrained fields and inline error highlights for missing required fields — without running any command.

**Why this priority**: Reduces errors before the operator ever runs the toolkit and speeds up manifest authoring. Secondary because it depends on editor tooling, not the runtime.

**Independent Test**: Open a blank YAML file in VS Code with a `$schema` header pointing to the schema file; confirm that omitting a required field produces an inline error and that typing `role:` triggers suggestions.

**Acceptance Scenarios**:

1. **Given** a YAML file with a `$schema` header pointing to the schema, **When** the developer omits `spec.node.advertisedHost`, **Then** the editor underlines the field as missing and the hover tooltip includes text stating it must never be inferred.
2. **Given** the developer types `role: ` in the spec section, **When** they trigger completion, **Then** the editor suggests only `central-bank` and `commercial-bank`.
3. **Given** the schema file is present in the repo, **When** a CI job validates a manifest against it, **Then** the exit code is non-zero for an invalid manifest.

---

### Edge Cases

- What happens when the manifest file does not exist or cannot be read?
- What happens when the file is syntactically invalid YAML (not parseable)?
- What happens when `apiVersion` is an unrecognized version (e.g., `cbweb3/v2`)?
- What happens when `spec.node.advertisedHost` is present but set to an empty string?
- What happens when `joinBundleRef` is present on a `mode: found` manifest (logically contradictory)?
- What happens when extra unknown fields are present in the manifest?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The schema MUST declare all required fields from the ParticipantDeployment spec and reject manifests missing any of them with a descriptive error identifying the missing field by its full path.
- **FR-002**: The schema MUST enforce enum constraints on `spec.role` (accepted: `central-bank`, `commercial-bank`), `spec.mode` (accepted: `found`, `join`), and `spec.scenario` (accepted: `a` only).
- **FR-003**: The field `spec.node.advertisedHost` MUST be required and its schema description MUST state it is never inferred from co-location; any manifest omitting it MUST be rejected with an error naming that field path explicitly.
- **FR-004**: The validator MUST report all validation failures in a single pass — the operator never needs to re-run to discover additional errors.
- **FR-005**: The validator MUST distinguish between a missing required field and a value failing an enum constraint, producing distinct error messages for each case.
- **FR-006**: The schema MUST be a valid JSON Schema (draft-07) document readable by standard YAML Language Server tooling, enabling editor autocompletion and inline error highlighting without additional plugin configuration.
- **FR-007**: Secrets (private keys, passphrases, credentials) MUST NOT be expressible as fields in the manifest schema; the structure must not accommodate them.
- **FR-008**: The schema MUST be versioned; manifests with an unrecognized `apiVersion` MUST be rejected with an error naming the field.
- **FR-009**: The schema MUST accept `environment` (values: `local`, `staging`, `prod`) as an optional field; its absence MUST NOT block validation.
- **FR-010**: The schema MUST accept `spec.relay.endpoint` as an optional field for relay configuration.
- **FR-011**: `joinBundleRef` MUST be accepted as an optional field in the schema; conditional-required enforcement (required only when `mode: join`) is out of scope for this task but MUST be documented in the schema description for that field.

### Key Entities

- **ParticipantDeployment manifest**: A self-contained YAML file describing a single participant's desired deployment state. Carries identity (role, mode), network topology (spoke, node addressing), security configuration (key provider, cert source), and image reference. Never contains private keys.
- **Validation result**: The outcome of schema validation — success (no findings) or failure (a list of errors, each identifying the field path and the nature of the violation).
- **JSON Schema (draft-07) document**: The machine-readable contract for the manifest format, used both at runtime for programmatic validation and at authoring time for editor tooling integration.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A valid manifest passes validation in under 100 milliseconds on a standard developer workstation.
- **SC-002**: Every required-field error names the exact field path (e.g., `spec.node.advertisedHost`) — no generic "required field missing" messages without a field name.
- **SC-003**: All validation failures in a manifest are reported in a single invocation; the operator never needs to re-run to discover additional errors.
- **SC-004**: The schema file is accepted by the VS Code YAML Language Server with no configuration changes beyond adding a `$schema` header to the manifest file.
- **SC-005**: The minimal valid manifest example from the provisioning spec (section 5.1 of the design document) passes validation without errors.
- **SC-006**: A manifest with every required field removed produces exactly one error per missing field, each naming the specific field path.

## Assumptions

- Operators run the validator on a standard developer workstation; no special runtime dependencies beyond the toolkit binary are assumed.
- The VS Code YAML Language Server (Red Hat extension) is the primary editor integration target; JetBrains and other editors are out of scope for now.
- `spec.image` accepts either the literal `build` or a registry image reference string; deep format validation of registry references is out of scope for this task.
- `spec.keyProvider` and `spec.certSource` accept URI-format strings (e.g., `kms://local-emulator`, `ca://...`, `self-signed`); deep URI parsing is out of scope.
- The schema covers only Scenario A manifests; Scenario B manifests are out of scope.
- The schema and its runtime validator are co-located in the same toolkit and must agree on required fields; keeping them in sync manually is acceptable for this task.
- Additional fields needed by `mode: join` (e.g., `joinBundleRef` conditionally required) are tracked as optional in this schema; their conditional enforcement belongs to a later task (TK-3 or later).
