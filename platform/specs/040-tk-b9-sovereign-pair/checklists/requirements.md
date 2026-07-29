# Specification Quality Checklist: TK-B9 — par soberano + liquidez cooperativa + seed-oracle

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
  - Q1 → **cauda soft do `found-spoke`** via `spec.pair`; o TK-B9 estende o found-spoke (TK-B7), sem
    modo standalone.
  - Q2 → **soberania estrita**: cada `apply` executa só o ato do CB corrente (CB-A propõe, CB-B
    confirma num found-spoke separado); nenhum run detém a chave da contraparte, inclusive em local.
- Decisões já firmadas no roadmap (não são clarificações): steps soft + idempotência on-chain
  (`PROPOSED→ACTIVE`); W-token por moeda (dedup); breaker opção A (sem mudança de lógica);
  `seed-oracle` local-only; commit-reveal cooperativo (cada CB só a sua moeda; relay casa
  `CommitMatched`); reuso de `SeedNewSovereignPair.s.sol`.
