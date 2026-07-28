# Specification Quality Checklist: TK-B8 — join (full node não-validador)

**Purpose**: Validate specification completeness and quality before proceeding to planning
**Created**: 2026-07-11
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

- **Clarificações resolvidas (Session 2026-07-11)**:
  - Fluxo canônico do `join` (roadmap §6) → **US4 removida** (sem `register-relay-bank`, sem
    `add-noc-agent`); a cadeia já é observada pelo relay desde o `found-spoke`.
  - Wire apenas endereços de spoke → o TK-B8 **não** altera o `emit-spoke-bundle` do TK-B7.
- Decisões já firmadas no roadmap (não são clarificações): banco = full node não-validador (CB =
  validador único); PKI do toolkit = apenas `gen-csr`; assinatura/registro = runtime.
