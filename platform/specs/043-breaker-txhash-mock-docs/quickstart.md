# Quickstart: Verifying Circuit-Breaker Transaction-Hash Visibility

**Feature**: `043-breaker-txhash-mock-docs` | **Date**: 2026-08-05

How to exercise and verify this feature once implemented. Two paths: automated (no stack required) and manual on a chain-wired stack.

---

## 1. Automated verification (no running stack)

```bash
# Backend — api-gateway service + handler contract tests
cd scenario-b
make test.api-gateway

# or directly
go test ./backend/services/api-gateway/internal/services/... -run CircuitBreaker -v
go test ./backend/services/api-gateway/internal/http/handlers/... -run CircuitBreaker -v
```

Expected: tests assert `tx_hash` is present and non-empty in all four V2 responses when the fake chain caller returns a hash, and **absent as a JSON key** (not `""`) when it does not.

```bash
# Frontend — governance portal
cd scenario-b/frontend
pnpm --filter governance test --run     # bare `vitest`; --run avoids watch mode
pnpm --filter governance type-check
pnpm --filter governance lint
pnpm --filter governance build
```

Expected: store-level tests show `tx_hash` reaching `cbStatus` from every response — including `proposeResume`, which rebuilds the object by hand — and the network-wide indicator derivation returning halted for any halted pair, operational for none, and never halted when the pair set is unknown.

> **Scope of automated frontend coverage.** The governance app runs Vitest in its default Node environment with no DOM and no component-testing library, and FR-026 forbids adding them. Rendering — the hash on the page, the copy control, the indicator itself — is verified manually in section 2, not here.

Per the managed policy, the full suite must also stay green:

```bash
cd scenario-b && make scenario-b.test-backend
```

---

## 2. Manual verification (chain-wired Scenario B stack)

### Bring the stack up

```bash
cd scenario-b
make scenario-b.up
make frontend-scenario-b
```

Confirm an AMM is wired for the pair under test — without it the breaker runs its off-chain path and correctly produces no hash.

### Walk the breaker lifecycle

Sign in to the Governance Portal as a Central Bank operator and open **Circuit Breaker**.

| Step | Action | Expect |
|---|---|---|
| 1 | Select a pair, click **Pause (1-of-N)** | State `HALTED`; a transaction hash appears with the action; copy control returns the full hash |
| 2 | Reload the page | The same hash is still shown — it is sourced from status, not from client memory |
| 3 | Check the chrome indicator and the Dashboard breaker card | Both report halted, agreeing with the page |
| 4 | If more than one pair exists, select an **operational** pair while the first stays halted | The page shows that pair operational, but the chrome indicator still reports halted — it is a network-wide claim (FR-012) |
| 5 | Click **Propose Resume** on the halted pair | State `RESUME_PENDING`; signature count `1/2`; a hash appears that is **different** from the displayed proposal ID |
| 6 | Sign in as a **second, distinct** Central Bank; open the same pair | The pair's latest hash is visible to this operator too |
| 7 | Click **Sign Resume (2-of-N)** | State returns to `LIVE`; a hash appears for the signing transaction |
| 8 | Wait for the indicator to refresh | With no pair halted, the indicator returns to operational |
| 9 | Cross-check any displayed hash against the ledger | The transaction exists and matches the action |

### Verify the raw hash on chain

```bash
cast tx <TX_HASH> --rpc-url http://localhost:<besu-port>
```

### No-chain behaviour

On a stack with no AMM wired for the pair, repeat steps 1 and 5. Both must still succeed and transition state; the UI must show a neutral "no on-chain reference" indication rather than an empty field, and the API must omit `tx_hash` entirely.

### No-pairs behaviour

On a stack where no pairs have been created, load the portal. The chrome indicator must **not** claim swaps are halted — it shows an unknown or operational condition and the layout is intact (FR-014). Create a pair and confirm the indicator starts tracking it on the next refresh.

---

## 3. Documentation verification

```bash
# No stale mock-vs-live claims remain
grep -rn -i 'mock' docs/user-manuals/scenario-a/governance.md \
                   docs/user-manuals/scenario-b/governance.md \
                   docs/user-manuals/scenario-b/bank.md \
                   docs/user-manuals/README.md

# The inert switch is gone from both bank templates
grep -rn 'VITE_USE_MOCKS' scenario-a/frontend/apps/bank/.env.example \
                          scenario-b/frontend/apps/bank/.env.example   # expect no matches

# The toggle is still declared where it IS consumed (governance apps) — these SHOULD match
grep -rn 'VITE_USE_MOCKS' scenario-b/frontend/apps/governance/.env.example
```

Cross-read each corrected statement against the behaviour it describes:

| Statement | Must say | Because |
|---|---|---|
| `scenario-a/governance.md:4` | live-backed, no selectable mock mode | nothing consumes `useMocks`; `mock-db` unimported |
| `scenario-b/governance.md:5` | a mock toggle exists and defaults to mock unless set to exactly `"false"` | consumed by `governance.api.ts`, `registry.api.ts` |
| `scenario-b/bank.md:5` + bank `SettingsPage.tsx` | agree with each other and with real behaviour | no bank code reads the flag |
| `docs/user-manuals/README.md` | attributes the live toggle to **Scenario B** Governance | that is where it is consumed |

Also confirm `docs/R2-H-2.md` carries the mis-scoping annotation (FR-021).

---

## 4. Acceptance summary

The feature is done when:

- Every breaker write action displays an on-chain hash the operator can copy, and it survives reload (FR-001–FR-010).
- The chrome indicator and the breaker page never disagree, including with several pairs and only one halted, and the indicator never asserts a halt it cannot substantiate (FR-011–FR-014).
- All four documentation statements match the code, and no inert variable remains (FR-015–FR-021).
- The two known gaps are recorded, not silently carried (FR-022, FR-023).
- No new dependency was introduced and every new file carries the licence header (FR-026, FR-027).
- Backend and frontend suites are green.
