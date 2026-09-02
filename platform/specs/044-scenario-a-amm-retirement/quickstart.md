# Quickstart: Verifying the Scenario A AMM Retirement

**Feature**: `044-scenario-a-amm-retirement` | **Date**: 2026-08-06

A deletion feature is verified differently from an additive one. The question is not "does the new thing work" but **"is everything that remains still correct, and is the removed thing really gone."** Both halves matter: a retirement that leaves a dangling reference breaks the build, and one that quietly removes a working control is a regression no test would flag if the test went with it.

All commands are run from `scenario-a/` unless stated otherwise.

---

## 1. Baseline — before touching anything

Record this first, so any later failure is attributable to the retirement rather than pre-existing.

```bash
cd scenario-a
make contracts.build && make contracts.test     # Foundry
make test.all                                   # compliance + auth + api-gateway
cd toolkit && go build ./... && go test ./...
```

```bash
cd scenario-a/frontend                          # npm workspaces, NOT pnpm
npm run build --workspace=governance
npm run build --workspace=bank
npm run lint --workspace=governance
npm run lint --workspace=bank
```

Save the results. Note any test that already fails and why — a pre-existing failure must not be silently inherited as if this feature caused it, and must not be used to excuse a real regression either.

Also record the licence gate, run from the repository root with **bash** (never zsh):

```bash
bash tools/check-license-headers.test.sh   # self-test
bash tools/check-license-headers.sh        # actual scan
```

---

## 2. The removal is complete

### Nothing in Scenario A still names the engine

```bash
cd scenario-a
grep -rn "AutomatedMarketMaker\|IAutomatedMarketMaker\|AMM_ADDRESS\|AMMAddress" \
  contracts backend toolkit frontend make \
  --include='*.sol' --include='*.go' --include='*.ts' --include='*.tsx' \
  --include='*.mk' --include='*.example' 2>/dev/null
```

Expected: **no matches.** Any hit is either a site the inventory missed or a deliberate exception that must be justified in the pull request.

```bash
# The deleted files really are gone
ls contracts/src/AutomatedMarketMaker.sol \
   contracts/src/interfaces/IAutomatedMarketMaker.sol \
   contracts/src/libraries/AutomatedMarketMakerLibrary.sol \
   contracts/script/AutomatedMarketMaker.s.sol \
   contracts/test/AutomatedMarketMaker.t.sol \
   backend/shared/blockchain/amm 2>&1        # expect: No such file or directory, for each
```

### No build entry point references a deleted script, and the live deploy is untouched

```bash
grep -rn "deploy-amm" make/            # expect no matches, in the target AND the .PHONY list
```

The live deployment path never deployed the engine, so verify it was **left alone** rather than that it still works without it:

```bash
# CBWeb3Spoke.s.sol is the live deploy (contracts.deploy-all → deploy-spoke-a/b) and must be unmodified
git diff --stat origin/develop...HEAD -- scenario-a/contracts/script/CBWeb3Spoke.s.sol   # expect: empty

# CBWeb3Hub.s.sol is legacy — its only target deliberately fails, so it is verified by compiling,
# not by running. Note this failure is pre-existing and NOT caused by the retirement.
make contracts.deploy-hub ; echo "exit=$? (expect 1, by design: 'hub-besu was removed')"
```

> Do not use `make -n contracts.deploy-all` as the check here. It depends only on `deploy-spoke-a` and `deploy-spoke-b`, neither of which ever referenced the AMM, so it would pass whether or not the removal was done correctly.

### Documentation no longer claims the engine is present or tested here

```bash
grep -rn -i "automatedmarketmaker\|AMM" README.md docs/ | grep -vi "scenario b\|scenario-b"
```

Read every remaining hit by eye — the grep only narrows the reading list. The distinction to apply is **not** scenario labelling, because most mentions already credit the other scenario correctly:

- **Must be corrected**: any claim that the engine is present in this repository or covered by its tests. In particular `docs/test-execution-plan.md` asserts Foundry unit and fuzz coverage of the AMM's invariants in four places — all false once the suite is deleted, and no build will flag it. Also the contract inventories with constructor arguments, and the runbook pointer to a deploy target that would reference a deleted script.
- **Must be preserved**: "used in Scenario B", "Constant-product AMM liquidity pool (Scenario B)", "the hub network is reserved for Scenario B (AMM)" and similar. These are accurate. A find-and-replace would corrupt them.

```bash
# The coverage claims are the easiest to miss and the most concrete — check them directly
grep -rn -i "amm" docs/test-execution-plan.md      # expect: no claim of AMM test coverage remains
```

---

## 3. Everything remaining still works

```bash
cd scenario-a
make contracts.build && make contracts.test
make test.all
cd toolkit && go build ./... && go test ./...
```

Expected: green, matching the baseline except for the deleted tests. The contract test count should fall by roughly 30 and the Go test count by 6 — **a falling count is the intended outcome here**, not a warning sign. Confirm the drop matches the deletions and nothing else went with them.

```bash
cd scenario-a/frontend
npm run build --workspace=governance && npm run build --workspace=bank
npm run lint  --workspace=governance && npm run lint  --workspace=bank
```

Expected: green, with **no new** warning or error against the baseline. Unused imports or types left behind by the deletion surface here.

### The other scenario is untouched

```bash
cd /home/xande/GoLedger/cbweb3-platform
git diff --name-only origin/develop...HEAD | grep '^scenario-b/'    # expect no matches
cd scenario-b && make contracts.test && make scenario-b.test-backend
```

Expected: no Scenario B file in the diff, and its suites green. The two swap engines are separate files; a Scenario B change here means the inventory was wrong.

### The contract definition did not move

```bash
cd /home/xande/GoLedger/cbweb3-platform
git diff --stat origin/develop...HEAD -- 'scenario-a/apis/proto/**' \
  'scenario-a/backend/shared/proto/**'
```

Expected: **empty.** This is FR-011. If anything appears here, the feature has attempted a contract change that cannot be regenerated in this environment.

---

## 4. The halt control still behaves identically

This is the part that a passing build does not prove. The database-only path is now the only path, so it must be exercised directly.

### Automated

```bash
cd scenario-a
make test.compliance      # breaker RPCs via the database path, with the chain client gone
make test.api-gateway     # gateway routes and handlers unchanged
```

Expected: the breaker tests pass without the on-chain test file, and no test needed rewriting to accommodate the removal — only the deleted on-chain test disappears.

### Manual, on a running stack

```bash
make spoke-all            # or the per-entity dev stack
```

Sign in to the Governance Portal as a governance operator, then:

| Step | Action | Expect |
|---|---|---|
| 1 | Read the status badge in the portal chrome | Reports the current halt state |
| 2 | Toggle the halt flag on, with a reason | Succeeds; state reports halted; the action is recorded |
| 3 | Reload the portal | Still halted — the flag is persisted, not held in memory |
| 4 | Toggle it off | Succeeds; state returns to not-halted |
| 5 | Check the service log at start-up | States that the on-chain breaker is unavailable and the database toggle is in use — and says so unconditionally now, not as one of two possibilities |
| 6 | Confirm no route reaches a breaker screen | The deleted governance screen was already unreachable; navigation is unchanged |

If a halt was recorded **before** the retirement, confirm it reads back identically after (D-2). This is the one case where the retirement could plausibly have changed observable state, so it is worth setting up deliberately rather than assuming.

### Bank portal — the dead call is gone

Open the bank dashboard with the browser network tab open. Expected: **no** swap-engine or pool request, and every rendered card identical to the baseline. The removed call's result was never displayed, so any visible difference means something other than the dead call was removed.

---

## 5. Acceptance summary

The feature is done when:

- Nothing in Scenario A source, scripts, configuration templates or build targets names the swap engine (FR-001…FR-009, SC-001).
- Scenario A contracts, services, toolkit and both portals build, test and lint green, with the test count down only by the deleted suites (SC-002, SC-007).
- Scenario B is untouched and green, proving the two engines were never shared (FR-009, SC-003).
- The interface contract definition and its generated code are unchanged (FR-011, SC-006).
- The halt control and its badge behave identically, and a pre-existing halt state survives (FR-010…FR-014, SC-004).
- The halt control is documented as a governance flag with no on-chain enforcement in Scenario A (FR-013, SC-005).
- No unreachable screen and no inert swap-engine variable remain, and the bank dashboard makes no synthetic call (FR-015…FR-020, SC-008, SC-009).
- The Scenario A governance manual describes one live data source, reconciled with feature 043 rather than overwriting it (FR-021, FR-026a).
- Workstream 2 is recorded as complete in **both** places (FR-023): locally in the untracked rescope plan, and in the change description where a reviewer can actually see it. The plan is still untracked afterwards — it is never committed to make the record verifiable.

---

## 6. The rescope plan is outside version control — check, don't fix

```bash
git ls-tree -r --name-only HEAD -- docs/r2-h-2-rescope-and-plan.md docs/R2-H-2.md   # expect: empty
git log --all --oneline -- docs/r2-h-2-rescope-and-plan.md                          # expect: empty
ls docs/r2-h-2-rescope-and-plan.md docs/R2-H-2.md                                   # expect: both present
```

Empty output from the first two commands with both files present on disk is the **intended** state: these are deliberately untracked working documents, held outside the repository by decision of the project owner. Nothing here is broken and nothing is blocked.

What follows from it:

- FR-023's update to the plan is a **local edit that no diff will show**. Make it anyway — the plan is the team's working record — then confirm `git status` still lists the file as untracked.
- **Do not `git add` the plan** to make the update verifiable. That reverses a deliberate decision in order to satisfy a checkbox.
- Put the reviewer-visible half of the record in the change description instead: state that Workstream 2 of the R2-H-2 rescope is complete.
- The net change removes substantially more than it adds, with no new dependency (FR-025, SC-010).
