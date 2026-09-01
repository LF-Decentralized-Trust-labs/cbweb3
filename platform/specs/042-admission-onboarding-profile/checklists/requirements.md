# Specification Quality Checklist: Admission Profile for Commercial-Bank Onboarding

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-08-03
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- All items pass. Open design decisions (Scenario A coupling approach; manifest admission-user
  required vs optional; governance residuals) are deliberately recorded as **Assumptions** with
  reasonable defaults rather than blocking [NEEDS CLARIFICATION] markers. They are surfaced formally
  in the `/speckit.clarify` phase for stakeholder confirmation before planning.

## Re-validation — 2026-08-04

Re-checked after the second codebase-verification pass, which added a clarification session and seven
requirements (FR-002a, FR-003a→b, FR-015a, FR-016, FR-017, SC-002a). All items above still pass.

**Amendment 1 (2026-09-01):** FR-015, FR-015a and FR-017 were stood down when the project lead chose to
keep the on-chain registration inside the Admission-authorized approval. The checklist items they were
recorded against are satisfied by the amendment rather than by those requirements.

- **Requirements are testable and unambiguous** — improved: the three previously unclassified routes
  (certificate issuance, registry read, participant provisioning) now have explicit owners, so the
  authorization contract no longer has silent defaults.
- **Scope is clearly bounded** — improved: the per-spoke nature of the profile is now stated, so the
  feature is not read as satisfying the rework document's cross-spoke requirement.
- **No implementation details leak into specification** — still holds. The file/line anchors from the
  verification pass live in `research.md`, `plan.md` and the contracts, not in `spec.md`.

Two decisions carry a recommendation but await project-lead confirmation (tracked as T045 and T047, both
reversible): whether certificate issuance stays with governance, and whether to introduce `ROLE_ADMISSION`
rather than reuse the dormant `ROLE_GOVERNANCE_OFFICER`. Neither blocks planning; both are recorded as
Assumptions with rationale.

## Re-validation — 2026-08-04 (third pass, full-artifact alignment)

A whole-spec pass reconciled every layer against the narrowed FR-002 / new FR-002a, and corrected the places
where earlier edits had not propagated. Items fixed, by layer:

- **spec.md** — User Story 1 still said Admission "drives the pipeline (… CSR …)"; User Story 3 omitted
  certificate issuance and operator provisioning from what governance retains; FR-004's list was closed
  rather than open; Key Entities and the "Governance keeps everything except onboarding" assumption
  contradicted FR-003a (the read views are *shared*, not moved); SC-001 and SC-007 were imprecise. All
  reconciled; FR-015/FR-015a moved into numeric order.
- **plan.md** — the Project Structure note still told the implementer to follow the `/api/v1/audit/logs`
  precedent, which the Fiber verification showed does not transfer; the "three invariants" and "~14 code
  areas" figures were stale. Two new Complexity Tracking entries added (group-guard relaxation; per-route
  frontend authorization), each with its rejected alternatives, as the constitution requires.
- **contracts/authorization-matrix.md** — the main tables marked in-group routes "unchanged", which would
  have invited leaving them unguarded after the group relaxation: the precise over-relaxation hazard INV-5
  exists to catch. Every cell now distinguishes "authority unchanged" from "code unchanged".
- **research.md** — R4's decision still carried a conditional that its own confirmation had resolved.
- **data-model.md** — the lifecycle diagram omitted the governance CSR-signing step; the no-key invariant
  did not mention the CA key.
- **quickstart.md** — prerequisites listed only the dual-grant operator, which by construction cannot detect
  a broken authorization boundary; three accounts are now required. The definition of done was missing
  SC-002a and understated SC-002 and SC-007.

**Checklist items above: all still pass.** Requirement-completeness improved most — the specification now
states the certificate-issuance boundary, the shared read surface, and the approved-≠-transactable semantics
explicitly, each of which was previously implied or contradicted.
