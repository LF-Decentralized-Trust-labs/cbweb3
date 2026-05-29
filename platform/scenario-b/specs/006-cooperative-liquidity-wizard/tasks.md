# Tasks: Wizard Frontend para Criação Cooperativa de Liquidez no AMM

**Feature**: `006-cooperative-liquidity-wizard`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data Model**: [data-model.md](data-model.md)
**Generated**: 2026-05-20 (updated — Phase B added; FR-028 T013 added)
**Total Tasks**: 13 | **Parallelizable**: 2 (T010 ∥ T007–T009; T011 ∥ T010) | **Sequential gates**: 6

---

## Dependency Graph (Story Completion Order)

```
Phase A — COMPLETE (T001 → T002 → T003 → T004 → T005 → T006)

Phase B — IN PROGRESS
  ┌──────────────────────────────────────────────────────────┐
  │  Governance track         Bank AMM track                 │
  │  T007 → T008 → T009       T010                          │
  │                  ↓         ↓                             │
  │                  └─────────┘                             │
  │                       ↓                                  │
  │                     T011 (Bank payment track)            │
  │                       ↓                                  │
  │                     T012 (verify both apps)              │
  │                       ↓                                  │
  │                     T013 (FR-028 route audit)            │
  └──────────────────────────────────────────────────────────┘
```

**Parallelism**: T010 (bank AMM track) has no dependency on T007–T009 — can be
worked concurrently. T011 depends only on T010 (auth store shape). T012 must come last.

**MVP Scope (Phase A)**: T001–T006 — wizard fully functional with 4 steps. COMPLETE.

**MVP Scope (Phase B)**: T007–T009 unblock governance app; T010–T011 unblock bank app;
T012 closes Phase B. All together fix the `HTTP 400 DEPRECATED_FIELDS / SIDE_REQUIRED`
errors introduced by the v4.0 runbook breaking changes.

---

## Phase 1: Foundational — Tipos, API e Store (COMPLETE)

> Objetivo: estabelecer a camada de dados e estado que todos os steps do wizard dependem.

- [X] T001 Extend `liquidity.types.ts` with new commit-reveal type definitions in `frontend/apps/governance/src/types/liquidity.types.ts`

- [X] T002 Extend `liquidity.api.ts` with three new commit API methods in `frontend/apps/governance/src/services/api/liquidity.api.ts`

- [X] T003 Extend `liquidity.store.ts` with commit state and actions in `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`

---

## Phase 2: US1/US2/US3/US4 — Wizard Component (COMPLETE)

> **Objetivo**: criar o componente `CooperativeLiquidityWizard` com os 4 steps do protocolo Commit-Reveal.
> **Critério de teste independente**: com backend mockado, um operador preenche Step 1 (mint & approve), avança para Step 2 (commit → retorna PENDING), entra no Step 3 (polling simula ACTIVE após 5s) e vê o Step 4 (success com LP IDs ou fallback). Caso de instant match: Step 2 retorna EXECUTED → salta direto para Step 4.

- [X] T004 [US1] Create `CooperativeLiquidityWizard.tsx` with all four steps (Mint & Approve, Commit, Monitor, Success) in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`

---

## Phase 3: US5 — Page Integration + Pending-Commit Banner (COMPLETE)

> **Objetivo**: integrar o wizard na `LiquidityManagementPage`, remover o formulário "Add Liquidity" quebrado e adicionar o banner de retomada de commit pendente.

- [X] T005 [US5] Modify `LiquidityManagementPage.tsx` to embed wizard and add pending-commit banner in `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

---

## Phase 4: Polish & Verificação (COMPLETE)

> Verificação final de type-safety e qualidade (governance app only, Phase A scope).

- [X] T006 Verify TypeScript typecheck and lint pass across all modified files in `frontend/apps/governance/`

---

## Phase 5: Governance — Breaking-Change Fixes: Types + API + Store (M7)

> **Objetivo**: corrigir os tipos e a camada de dados da governance app para os breaking changes
> do runbook v4.0. `MintAndApproveRequest` passa de dois campos (`amount_a`, `amount_b`) para
> um único `amount`. Novo tipo `ApproveAmmRequest` com `amount` + `side?` suporta o novo
> endpoint `POST /amm/token/approve-amm`. Store ganha a action `approveAmm`.
>
> **Critério de teste independente**: após T007, `pnpm --filter @cbweb3/governance tsc --noEmit`
> deve retornar zero erros. `grep -r "amount_a\|amount_b"` nas três pastas alvo deve retornar
> zero hits.

- [X] T007 Modify governance types, API, and store to adopt single-amount MintAndApproveRequest and add ApproveAmmRequest in `frontend/apps/governance/src/types/liquidity.types.ts`, `frontend/apps/governance/src/services/api/liquidity.api.ts`, `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`

---

## Phase 6: Governance — Standalone MintAndApprove Panel (M8)

> **Objetivo**: atualizar o formulário MintAndApprove standalone (fora do wizard) na
> `LiquidityManagementPage` para usar o novo campo `amount` único no lugar de `amount_a`
> e `amount_b`.
>
> **Critério de teste independente**: `grep -n "approveAmountA\|approveAmountB\|approve_amount_a\|approve_amount_b"` em `LiquidityManagementPage.tsx` retorna zero hits; form still submits correctly.

- [X] T008 Modify `LiquidityManagementPage.tsx` standalone MintAndApprove panel: replace approveAmountA/approveAmountB with single approveAmount field in `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

---

## Phase 7: Governance — Wizard Step 1 Rewrite + G5-Cross Block (M9)

> **Objetivo**: atualizar o Step 1 do wizard para usar `amount` único e adicionar o bloco
> G5-cross colapsível para setup de ambiente fresh. O bloco G5-cross fornece botões
> independentes G5.1 (mintAndApprove para CB-A) e G5.2 (approveAmm side B).
>
> **Critério de teste independente**: `grep -n "mintAmountA\|mintAmountB\|wizard_amount_a\|wizard_amount_b"` em `CooperativeLiquidityWizard.tsx` retorna zero hits; G5-cross block hidden by default; Step 1 submit still advances to Step 2.

- [X] T009 Modify `CooperativeLiquidityWizard.tsx` Step 1: replace two-amount form with single amount, add G5-cross collapsible section with independent G5.1/G5.2 buttons in `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`

---

## Phase 8: Bank App — AMM Types + Store + AMMTradingPage (M10)

> **Objetivo**: corrigir o app bank para os breaking changes do endpoint approve-amm.
> `ApproveAmmRequest` passa de `{amount_a, amount_b}` para `{amount, side: "A"|"B"}`.
> `PoolStatus` é extendido com campos de lifecycle. O formulário de approve na
> `AMMTradingPage` é atualizado e um pool-status gate bloqueia swaps quando `pool_status !== "ACTIVE"`.
>
> **Critério de teste independente**: `grep -r "amount_a\|amount_b" frontend/apps/bank/src/types/amm-v2.types.ts frontend/apps/bank/src/features/amm frontend/apps/bank/src/pages/AMMTradingPage.tsx` retorna zero hits; swap button disabled with inline message when pool_status is EMPTY or PENDING_COUNTERPART.
>
> **Paralelismo**: este phase é independente de T007–T009 e pode ser executado em paralelo.

- [X] T010 [P] Modify bank AMM types, store action signature, and AMMTradingPage approve form + swap pool-status gate in `frontend/apps/bank/src/types/amm-v2.types.ts`, `frontend/apps/bank/src/features/amm/amm-v2.store.ts`, `frontend/apps/bank/src/pages/AMMTradingPage.tsx`

---

## Phase 9: Bank App — Payment Types + API + Store + Pages (M11)

> **Objetivo**: corrigir os três endpoints de pagamento para os breaking changes do runbook v4.0.
> `RegisterDepositRequest`, `RequestEscrowRequest`, `RequestRedeemRequest` recebem novos shapes.
> Páginas DepositsPage, EscrowsPage e RedeemsPage injetam automaticamente os campos `requester_*`
> e introduzem UX específica (banner pós-escrow, toggle Zeto).
>
> **Critério de teste independente**: `grep -r "CreateAmountRequest" frontend/apps/bank/src/` retorna zero hits; EscrowsPage form tem input `deposit_id` (não `amount`); Zeto field ausente do DOM quando toggle está desligado; `pnpm --filter @cbweb3/bank tsc --noEmit` retorna zero erros.

- [X] T011 Modify bank payment types, API, store, DepositsPage, EscrowsPage, RedeemsPage to adopt new request shapes with auto-injected requester fields in `frontend/apps/bank/src/types/payment.types.ts`, `frontend/apps/bank/src/services/api/payment.api.ts`, `frontend/apps/bank/src/stores/payment.store.ts`, `frontend/apps/bank/src/pages/DepositsPage.tsx`, `frontend/apps/bank/src/pages/EscrowsPage.tsx`, `frontend/apps/bank/src/pages/RedeemsPage.tsx`

---

## Phase 10: Polish & Verificação Fase B (M12)

> Verificação final de type-safety e qualidade em ambas as apps (governance + bank).
> Deve ser executado após T007–T011 estarem todos completos.

- [X] T012 Run `tsc --noEmit` and lint in both `frontend/apps/governance/` and `frontend/apps/bank/`, fix any type errors from the Phase B payload changes

---

## Phase 11: FR-028 — CB Governance Payment Routes Verification

> **Objetivo**: garantir que a governance frontend consome os 7 novos endpoints de pagamento CB
> registrados em `router.go` em 2026-05-20 (branch `feature/frontend-liquidity`) com as rotas
> canônicas corretas. Nenhuma implementação de backend é necessária — o backend já está completo.
> Esta fase é uma auditoria de integração frontend ↔ backend.
>
> **Critério de teste independente**: `grep -rn "/payment/deposit/register\|/payment/exchange/request\|/payment/redeem/request"` em `frontend/apps/governance/src/` retorna zero hits; `payment.api.ts` referencia apenas os 7 caminhos canônicos `/api/v1/payments/...`.

- [ ] T013 Verify governance `payment.api.ts` calls the 7 canonical CB payment routes (FR-028) and confirm no stale paths exist in `frontend/apps/governance/src/services/api/payment.api.ts`

---

## Task Details

---

### T001–T006 — Phase A (COMPLETE)

All Phase A tasks are complete. See git history for implementation details.

Summary of completed work:
- **T001**: Extended `liquidity.types.ts` — CommitSide, CommitStatus, PoolLifecycleStatus, CommitRequest, CommitResult, CommitsListResponse; PoolStatus extended with pool_status?, fee_rate_bps?, total_lp_count?, pending_commits?
- **T002**: Added `commitLiquidity`, `listCommits`, `cancelCommit` to `liquidity.api.ts`
- **T003**: Added activeCommit, commitStatus, commitError state + submitCommit, cancelActiveCommit, clearCommit actions to `liquidity.store.ts`
- **T004**: Created `CooperativeLiquidityWizard.tsx` with StepIndicator + 4 steps (Mint & Approve → Commit → Monitor → Success); instant-match routing; 5s polling; 1s countdown
- **T005**: Removed broken "Add Liquidity" form from `LiquidityManagementPage`; embedded wizard; added pending-commit banner with provider_id filter
- **T006**: TypeScript + lint clean in governance app

---

### T007 — Governance types + API + store: adopt single-amount shape (M7)

**Title**: Rewrite `MintAndApproveRequest` to `{amount, recipient?}`; add `ApproveAmmRequest`; add `approveAmm` action

**Description**: Three-file change that fixes the governance app's `HTTP 400 DEPRECATED_FIELDS`
errors. `MintAndApproveRequest` loses `amount_a` and `amount_b` — replaced with single
`amount: string`. New `ApproveAmmRequest = {amount: string; side?: "A" | "B"}` maps to
`POST /amm/token/approve-amm`. Store gains `approveAmm(amount, side?)` action. This task
is the prerequisite for T008 and T009.

**Dependencies**: none (replaces old types; existing T001–T003 additions remain valid)

**Acceptance criteria**:
- `pnpm --filter @cbweb3/governance tsc --noEmit` zero errors
- `grep -r "amount_a\|amount_b" frontend/apps/governance/src/types frontend/apps/governance/src/services frontend/apps/governance/src/features/liquidity/liquidity.store.ts` → zero hits
- `ApproveAmmRequest` exported from `liquidity.types.ts`
- `liquidityApi.approveAmm(payload)` calls `POST /api/v2/amm/token/approve-amm`
- `useLiquidityStore` has `approveAmm(amount, side?)` action

**Files**:
- Modify: `frontend/apps/governance/src/types/liquidity.types.ts`
- Modify: `frontend/apps/governance/src/services/api/liquidity.api.ts`
- Modify: `frontend/apps/governance/src/features/liquidity/liquidity.store.ts`

**Implementation hints**:

**`liquidity.types.ts`** — replace `MintAndApproveRequest` and add `ApproveAmmRequest`:

```typescript
// BEFORE (remove):
export interface MintAndApproveRequest {
  amount_a: string;
  amount_b: string;
  recipient?: string;
}

// AFTER (replace with):
export interface MintAndApproveRequest {
  amount: string;
  recipient?: string;
}

// ADD after MintAndApproveRequest:
export interface ApproveAmmRequest {
  amount: string;
  side?: "A" | "B";
}
```

**`liquidity.api.ts`** — `mintAndApprove` signature does not change (it already accepts
`MintAndApproveRequest`); only callers change. Add `approveAmm` method:

```typescript
// ADD to liquidityApi object, after mintAndApprove:
approveAmm: async (payload: ApproveAmmRequest): Promise<void> => {
  await httpClientV2.post("/amm/token/approve-amm", payload);
},
```

Add `ApproveAmmRequest` to the import block at the top of the file.

**`liquidity.store.ts`** — extend type definition and implementation:

```typescript
// ADD to LiquidityStore type definition:
approveAmm: (amount: string, side?: "A" | "B") => Promise<void>;

// ADD to create<LiquidityStore>((set) => ({ ... })), after mintAndApprove:
approveAmm: async (amount, side) => {
  set({ status: "loading", error: null });
  try {
    await liquidityApi.approveAmm({ amount, side });
    set({ status: "idle" });
  } catch (error) {
    set({
      status: "error",
      error: extractApiError(error, "Unable to approve AMM"),
    });
  }
},
```

Add `ApproveAmmRequest` to the import from `../../types/liquidity.types`.

**Spot-check**:
```bash
grep -r "amount_a\|amount_b" \
  frontend/apps/governance/src/types \
  frontend/apps/governance/src/services \
  frontend/apps/governance/src/features/liquidity/liquidity.store.ts
# Expected: 0 hits
```

---

### T008 — Governance `LiquidityManagementPage`: standalone MintAndApprove panel (M8)

**Title**: Replace `approveAmountA`/`approveAmountB` state with single `approveAmount` in the standalone MintAndApprove panel

**Description**: The standalone "Mint & Approve" card in `LiquidityManagementPage` (separate from
the wizard) still uses two-amount state and calls `mintAndApprove({amount_a, amount_b, recipient})`.
Update it to use a single `amount` field consistent with the new `MintAndApproveRequest` shape.

**Dependencies**: T007

**Acceptance criteria**:
- `grep -n "approveAmountA\|approveAmountB\|approve_amount_a\|approve_amount_b" frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx` → zero hits
- Single `<Input id="approve_amount" />` labelled "Amount" renders in the MintAndApprove panel
- `handleMintAndApprove` calls `mintAndApprove({ amount: approveAmount, recipient: mintRecipient || undefined })`
- All other cards (Remove Liquidity, LP Positions, Pool Status, wizard trigger) unchanged

**Files**:
- Modify: `frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx`

**Implementation hints**:

1. **Remove** the two separate state variables:
   ```typescript
   // REMOVE:
   const [approveAmountA, setApproveAmountA] = useState("");
   const [approveAmountB, setApproveAmountB] = useState("");

   // ADD:
   const [approveAmount, setApproveAmount] = useState("");
   ```

2. **Update** `handleMintAndApprove`:
   ```typescript
   // BEFORE:
   await mintAndApprove({ amount_a: approveAmountA, amount_b: approveAmountB, recipient: mintRecipient || undefined });

   // AFTER:
   await mintAndApprove({ amount: approveAmount, recipient: mintRecipient || undefined });
   ```

3. **Replace** the two inputs in the form with one:
   ```tsx
   {/* REMOVE both Input blocks for amount_a and amount_b */}

   {/* ADD: */}
   <div className="space-y-1">
     <Label htmlFor="approve_amount">Amount</Label>
     <Input
       id="approve_amount"
       value={approveAmount}
       onChange={(e) => setApproveAmount(e.target.value)}
       placeholder="e.g. 1000"
       required
     />
   </div>
   ```

4. **Update** the submit guard (if any):
   ```typescript
   // BEFORE (if it exists):
   disabled={!approveAmountA || !approveAmountB || status === "loading"}

   // AFTER:
   disabled={!approveAmount || status === "loading"}
   ```

---

### T009 — Governance wizard Step 1: single amount + G5-cross collapsible block (M9)

**Title**: Replace two-amount fields with single `mintAmount` in Step 1; add G5-cross setup block

**Description**: Step 1 of the wizard currently uses `mintAmountA` and `mintAmountB` state.
Replace with single `mintAmount`. Additionally, add a collapsible "G5-cross" section below
the main Step 1 form — only visible when a toggle is enabled — that lets the operator mint
TOKEN_B to CB-A (G5.1) and approve TOKEN_B for the AMM from CB-A's side (G5.2) as part of
a fresh-environment bootstrap. Both G5 buttons are independent of each other and independent
of the main Step 1 submit.

**Dependencies**: T007, T008

**Acceptance criteria**:
- `grep -n "mintAmountA\|mintAmountB\|wizard_amount_a\|wizard_amount_b" frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx` → zero hits
- Single `<Input id="wizard_amount" />` renders in Step 1
- Main Step 1 submit calls `mintAndApprove({ amount: mintAmount, recipient: mintRecipient || undefined })` and advances to Step 2 on success
- G5-cross block hidden by default (`showG5Cross === false`)
- G5-cross block expands when toggle is checked
- G5.1 button calls `mintAndApprove({ amount: g5Amount, recipient: g5SignerAddress })` — disabled when `g5Amount` is empty OR `g5SignerAddress` does not match `/^0x[0-9a-fA-F]{40}$/`
- G5.2 button calls `approveAmm(g5Amount, "B")` — disabled when `g5Amount` is empty; independent of G5.1 completion
- Both G5 buttons disabled while `status === "loading"`
- G5-specific errors shown inline (separate from main form error)
- After G5.1 succeeds: `g51Done = true` and "Done" badge rendered next to G5.1 button
- After G5.2 succeeds: `g52Done = true` and "Done" badge rendered next to G5.2 button
- Deactivating toggle resets `g5Amount`, `g5SignerAddress`, `g51Done`, `g52Done` to defaults
- Main Step 1 submit unaffected by G5-cross state

**Files**:
- Modify: `frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx`

**Implementation hints**:

**State changes** (inside the wizard component, wherever Step 1 form state lives):

```typescript
// REMOVE:
const [mintAmountA, setMintAmountA] = useState("");
const [mintAmountB, setMintAmountB] = useState("");

// ADD:
const [mintAmount, setMintAmount] = useState("");

// ADD (G5-cross state):
const [showG5Cross, setShowG5Cross] = useState(false);
const [g5Amount, setG5Amount] = useState("");
const [g5SignerAddress, setG5SignerAddress] = useState("");
const [g51Done, setG51Done] = useState(false);
const [g52Done, setG52Done] = useState(false);
const [g5Error, setG5Error] = useState<string | null>(null);
```

**Store selectors** — add `approveAmm` alongside existing `mintAndApprove`:

```typescript
const approveAmm = useLiquidityStore((state) => state.approveAmm);
```

**`handleMintApprove`** — update payload:

```typescript
// BEFORE:
await mintAndApprove({ amount_a: mintAmountA, amount_b: mintAmountB, recipient: mintRecipient || undefined });

// AFTER:
await mintAndApprove({ amount: mintAmount, recipient: mintRecipient || undefined });
```

**Single amount input** (replace two inputs):

```tsx
{/* REMOVE both wizard_amount_a and wizard_amount_b inputs */}

{/* ADD: */}
<div className="space-y-1">
  <Label htmlFor="wizard_amount">Amount</Label>
  <Input
    id="wizard_amount"
    value={mintAmount}
    onChange={(e) => setMintAmount(e.target.value)}
    placeholder="e.g. 1000"
    required
  />
</div>
```

**G5-cross block** — add below the main Step 1 form fields, before the submit button:

```tsx
{/* G5-cross collapsible section */}
<div className="border-t pt-4 mt-4">
  <label className="flex items-center gap-2 cursor-pointer text-sm font-medium">
    <input
      type="checkbox"
      checked={showG5Cross}
      onChange={(e) => {
        setShowG5Cross(e.target.checked);
        if (!e.target.checked) {
          setG5Amount("");
          setG5SignerAddress("");
          setG51Done(false);
          setG52Done(false);
          setG5Error(null);
        }
      }}
    />
    Is this a fresh environment? (G5-cross setup required)
  </label>

  {showG5Cross && (
    <div className="mt-3 space-y-3 pl-4 border-l-2 border-muted">
      <div className="space-y-1">
        <Label htmlFor="g5_amount">G5 Amount</Label>
        <Input
          id="g5_amount"
          value={g5Amount}
          onChange={(e) => setG5Amount(e.target.value)}
          placeholder="e.g. 1000"
        />
      </div>

      <div className="space-y-1">
        <Label htmlFor="g5_signer_address">CB-A Signer Address</Label>
        <Input
          id="g5_signer_address"
          value={g5SignerAddress}
          onChange={(e) => setG5SignerAddress(e.target.value)}
          placeholder="0x..."
        />
        {g5SignerAddress && !/^0x[0-9a-fA-F]{40}$/.test(g5SignerAddress) && (
          <p className="text-xs text-destructive">Must be a valid 0x address (42 chars)</p>
        )}
      </div>

      {g5Error && <p className="text-sm text-destructive">{g5Error}</p>}

      <div className="flex gap-2 flex-wrap">
        {/* G5.1 — Mint TOKEN_B to CB-A */}
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={
              !g5Amount ||
              !/^0x[0-9a-fA-F]{40}$/.test(g5SignerAddress) ||
              status === "loading"
            }
            onClick={async () => {
              setG5Error(null);
              await mintAndApprove({ amount: g5Amount, recipient: g5SignerAddress });
              const s = useLiquidityStore.getState();
              if (s.status === "idle") {
                setG51Done(true);
              } else {
                setG5Error(s.error ?? "G5.1 failed");
              }
            }}
          >
            Mint TOKEN_B to CB-A
          </Button>
          {g51Done && <Badge variant="secondary">Done</Badge>}
        </div>

        {/* G5.2 — Approve TOKEN_B for AMM (side B) */}
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={!g5Amount || status === "loading"}
            onClick={async () => {
              setG5Error(null);
              await approveAmm(g5Amount, "B");
              const s = useLiquidityStore.getState();
              if (s.status === "idle") {
                setG52Done(true);
              } else {
                setG5Error(s.error ?? "G5.2 failed");
              }
            }}
          >
            Approve TOKEN_B for AMM (CB-A)
          </Button>
          {g52Done && <Badge variant="secondary">Done</Badge>}
        </div>
      </div>
    </div>
  )}
</div>
```

> **Note**: `Badge` is already available from `@cbweb3/ui`. Import if not already in the file.
> G5.2 is intentionally independent of G5.1 — no completion guard between them.

---

### T010 — Bank app AMM: types + store + AMMTradingPage (M10)

**Title**: Rewrite `ApproveAmmRequest` to `{amount, side}`; extend `PoolStatus`; update store action and approve form; add swap pool-status gate

**Description**: The bank app's approve-amm calls fail with `HTTP 400 SIDE_REQUIRED` because the
old payload has `{amount_a, amount_b}` instead of `{amount, side}`. Update the type, the store
action signature, and the `AMMTradingPage` approve form. Additionally add a pool-status gate on
the swap submit button — swaps are disabled when `pool_status !== "ACTIVE"`. This task is
independent of T007–T009 (different app) and can be done in parallel.

**Dependencies**: none

**Acceptance criteria**:
- `grep -r "amount_a\|amount_b" frontend/apps/bank/src/types/amm-v2.types.ts frontend/apps/bank/src/features/amm frontend/apps/bank/src/pages/AMMTradingPage.tsx` → zero hits
- Approve form has single `<Input id="approve_amount" />` + side selector (options "A" / "B")
- Side not selected → approve submit button disabled
- Swap submit button disabled when `poolStatus?.pool_status !== "ACTIVE"` and `pool_status` is present
- Inline message "Pool is not active. Swaps are currently unavailable." shown when gate is active
- Quote form (getQuote) remains active regardless of pool_status
- `pnpm --filter @cbweb3/bank tsc --noEmit` zero errors

**Files**:
- Modify: `frontend/apps/bank/src/types/amm-v2.types.ts`
- Modify: `frontend/apps/bank/src/features/amm/amm-v2.store.ts`
- Modify: `frontend/apps/bank/src/pages/AMMTradingPage.tsx`

**Implementation hints**:

**`amm-v2.types.ts`** — rewrite `ApproveAmmRequest` and extend `PoolStatus`:

```typescript
// BEFORE (remove):
export interface ApproveAmmRequest {
  amount_a: string;
  amount_b: string;
}

// AFTER:
export interface ApproveAmmRequest {
  amount: string;
  side: "A" | "B";
}

// Extend PoolStatus with lifecycle fields (add optional fields — preserve all existing):
export interface PoolStatus {
  // ... existing fields ...
  pool_status?: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  fee_rate_bps?: number;
  total_lp_count?: number;
}
```

**`amm-v2.store.ts`** — update `approveAmm` action signature and body:

```typescript
// In LiquidityStore type (or equivalent store type):
// BEFORE:
approveAmm: (amount_a: string, amount_b: string) => Promise<void>;
// AFTER:
approveAmm: (amount: string, side: "A" | "B") => Promise<void>;

// In implementation:
// BEFORE:
approveAmm: async (amount_a, amount_b) => {
  ...
  await ammV2Api.approveAmm({ amount_a, amount_b });
  ...
},
// AFTER:
approveAmm: async (amount, side) => {
  ...
  await ammV2Api.approveAmm({ amount, side });
  ...
},
```

> Check the actual state field names in the existing store (e.g. `approveStatus`, `status`, etc.)
> and use whatever names are already there. Only the action signature and payload object change.

**`AMMTradingPage.tsx`** — update approve form:

```typescript
// REMOVE:
const [approveAmountA, setApproveAmountA] = useState("");
const [approveAmountB, setApproveAmountB] = useState("");

// ADD:
const [approveAmount, setApproveAmount] = useState("");
const [approveSide, setApproveSide] = useState<"A" | "B" | "">("");
```

Update `handleApproveAmm`:

```typescript
// BEFORE:
await approveAmm(approveAmountA, approveAmountB);

// AFTER:
if (!approveSide) return;
await approveAmm(approveAmount, approveSide as "A" | "B");
```

Replace two amount inputs with one amount input + side radio:

```tsx
{/* REMOVE both approveAmountA and approveAmountB inputs */}

{/* ADD: */}
<div className="space-y-1">
  <Label htmlFor="approve_amount">Amount</Label>
  <Input
    id="approve_amount"
    value={approveAmount}
    onChange={(e) => setApproveAmount(e.target.value)}
    placeholder="e.g. 1000"
  />
</div>

<div className="space-y-1">
  <Label>Side</Label>
  <div className="flex gap-4">
    <label className="flex items-center gap-1 text-sm cursor-pointer">
      <input
        type="radio"
        name="approveSide"
        value="A"
        checked={approveSide === "A"}
        onChange={() => setApproveSide("A")}
      />
      Token A
    </label>
    <label className="flex items-center gap-1 text-sm cursor-pointer">
      <input
        type="radio"
        name="approveSide"
        value="B"
        checked={approveSide === "B"}
        onChange={() => setApproveSide("B")}
      />
      Token B
    </label>
  </div>
</div>
```

Update approve submit button:

```tsx
disabled={!approveAmount || !approveSide || approveStatus === "loading"}
```

**Pool-status gate on swap submit** — add inline message and update disabled condition:

```tsx
{/* Add above swap submit button: */}
{poolStatus?.pool_status && poolStatus.pool_status !== "ACTIVE" && (
  <p className="text-sm text-destructive">
    Pool is not active. Swaps are currently unavailable.
  </p>
)}

{/* Update swap submit button disabled condition: */}
disabled={
  swapStatus === "loading" ||
  (poolStatus?.pool_status !== undefined && poolStatus.pool_status !== "ACTIVE")
}
```

> `poolStatus` should already be in scope via `useAmmV2Store` — verify the selector name.

---

### T011 — Bank app payment layer: deposit/escrow/redeem payload updates (M11)

**Title**: Adopt new `RegisterDepositRequest`, `RequestEscrowRequest`, `RequestRedeemRequest`; auto-inject requester fields; update DepositsPage, EscrowsPage, RedeemsPage UX

**Description**: Three payment endpoints changed shape in the v4.0 runbook. This task updates
types, the API layer, the store, and all three pages. Key behaviors:
- `registerDeposit`: reads `wallet_address` from auth store and `VITE_PALADIN_IDENTITY` from
  env — never entered manually by the user.
- `requestEscrow`: uses `deposit_id` (first APPROVED deposit) instead of `amount`.
- `requestRedeem`: same auto-inject pattern as deposit; optional Zeto hash via toggle.
- All three pages show a warning and disable submit if `wallet_address` is missing.

**Dependencies**: T010 (auth store shape — verify `wallet_address` field name in `auth.store.ts` before coding; it may be camelCase `walletAddress`)

**Acceptance criteria**:
- `grep -r "CreateAmountRequest" frontend/apps/bank/src/` → zero hits
- EscrowsPage renders `<Input id="deposit_id" />`, not an amount input
- Zeto field `<Input id="zeto_transfer_tx_hash" />` absent from DOM when toggle is off
- Payload never includes `zeto_transfer_tx_hash` with value `""` or `undefined`
- DepositsPage and RedeemsPage show "Onboarding incomplete — wallet address unavailable" and disable submit when `wallet_address` is null
- Post-escrow banner shown when latest escrow `status === PaymentStatus.APPROVED`
- `pnpm --filter @cbweb3/bank tsc --noEmit` zero errors

**Files**:
- Modify: `frontend/apps/bank/src/types/payment.types.ts`
- Modify: `frontend/apps/bank/src/services/api/payment.api.ts`
- Modify: `frontend/apps/bank/src/stores/payment.store.ts`
- Modify: `frontend/apps/bank/src/pages/DepositsPage.tsx`
- Modify: `frontend/apps/bank/src/pages/EscrowsPage.tsx`
- Modify: `frontend/apps/bank/src/pages/RedeemsPage.tsx`

**Implementation hints**:

**`payment.types.ts`** — add new request interfaces, remove `CreateAmountRequest`:

```typescript
// ADD:
export interface RegisterDepositRequest {
  requester_besu_address: string;
  requester_paladin_identity: string;
  amount: string;
}

export interface RequestEscrowRequest {
  deposit_id: string;
}

export interface RequestRedeemRequest {
  requester_besu_address: string;
  requester_paladin_identity: string;
  amount: string;
  zeto_transfer_tx_hash?: string;
}

// REMOVE (after confirming zero callers remain):
// export interface CreateAmountRequest { ... }
```

**`payment.api.ts`** — update function parameter types:

```typescript
// Update each function signature (HTTP body passes through unchanged):
registerDeposit: async (payload: RegisterDepositRequest): Promise<Deposit> => { ... }
requestEscrow:   async (payload: RequestEscrowRequest):   Promise<Escrow>  => { ... }
requestRedeem:   async (payload: RequestRedeemRequest):   Promise<Redeem>  => { ... }
```

Update imports to use new types instead of `CreateAmountRequest`.

**`payment.store.ts`** — update `PaymentStore` type and action implementations:

```typescript
// Type changes:
registerDeposit: (requester_besu_address: string, requester_paladin_identity: string, amount: string) => Promise<void>;
requestEscrow:   (deposit_id: string) => Promise<void>;
requestRedeem:   (requester_besu_address: string, requester_paladin_identity: string, amount: string, zeto_transfer_tx_hash?: string) => Promise<void>;

// Implementation — registerDeposit:
registerDeposit: async (requester_besu_address, requester_paladin_identity, amount) => {
  set({ depositStatus: "loading", depositError: null });
  try {
    const result = await paymentApi.registerDeposit({ requester_besu_address, requester_paladin_identity, amount });
    set((state) => ({ deposits: [...state.deposits, result], depositStatus: "idle" }));
  } catch (error) {
    set({ depositStatus: "error", depositError: extractApiError(error, "Deposit failed") });
  }
},

// Implementation — requestEscrow:
requestEscrow: async (deposit_id) => {
  set({ escrowStatus: "loading", escrowError: null });
  try {
    const result = await paymentApi.requestEscrow({ deposit_id });
    set((state) => ({ escrows: [...state.escrows, result], escrowStatus: "idle" }));
  } catch (error) {
    set({ escrowStatus: "error", escrowError: extractApiError(error, "Escrow request failed") });
  }
},

// Implementation — requestRedeem:
requestRedeem: async (requester_besu_address, requester_paladin_identity, amount, zeto_transfer_tx_hash?) => {
  set({ redeemStatus: "loading", redeemError: null });
  try {
    const result = await paymentApi.requestRedeem({
      requester_besu_address,
      requester_paladin_identity,
      amount,
      ...(zeto_transfer_tx_hash ? { zeto_transfer_tx_hash } : {}),
    });
    set((state) => ({ redeems: [...state.redeems, result], redeemStatus: "idle" }));
  } catch (error) {
    set({ redeemStatus: "error", redeemError: extractApiError(error, "Redeem request failed") });
  }
},
```

> Adjust state field names (`depositStatus`, `escrowStatus`, `redeemStatus`, etc.) to match
> whatever names are already in the store.

**`DepositsPage.tsx`** — auto-inject requester fields, add missing-wallet guard:

```typescript
// Read auth store (verify exact field name first):
const walletAddress = useAuthStore((state) => state.profile?.wallet_address ?? null);
const paladinIdentity = import.meta.env.VITE_PALADIN_IDENTITY as string;

const canSubmit = !!walletAddress && !!amount;

// Handler:
const handleRegisterDeposit = async (e: FormEvent) => {
  e.preventDefault();
  if (!walletAddress) return;
  await registerDeposit(walletAddress, paladinIdentity, amount);
};
```

In JSX, add warning and update submit button:
```tsx
{!walletAddress && (
  <p className="text-sm text-destructive">
    Onboarding incomplete — wallet address unavailable. Contact your system administrator.
  </p>
)}

<Button type="submit" disabled={!canSubmit || depositStatus === "loading"}>
  Register Deposit
</Button>
```

**`EscrowsPage.tsx`** — replace amount with deposit_id input, add post-escrow banner:

```typescript
// REMOVE:
const [amount, setAmount] = useState("");

// ADD:
const [depositId, setDepositId] = useState("");

// Derive default depositId from store:
const deposits = usePaymentStore((state) => state.deposits);
const firstApprovedDeposit = deposits.find((d) => d.status === PaymentStatus.APPROVED);

useEffect(() => {
  if (firstApprovedDeposit && !depositId) {
    setDepositId(firstApprovedDeposit.id);
  }
}, [firstApprovedDeposit]);

// Handler:
const handleRequestEscrow = async (e: FormEvent) => {
  e.preventDefault();
  await requestEscrow(depositId);
};

// Post-escrow banner (add near top of page):
const escrows = usePaymentStore((state) => state.escrows);
const latestEscrow = escrows[escrows.length - 1];
const showApprovedBanner = latestEscrow?.status === PaymentStatus.APPROVED;
```

In JSX:
```tsx
{showApprovedBanner && (
  <Card className="border-green-500 bg-green-50 dark:bg-green-950/20">
    <CardContent className="py-3 flex items-center justify-between">
      <p className="text-sm font-medium">
        Your tCeBM has been issued — next: Approve AMM spending
      </p>
      <Button size="sm" variant="outline" onClick={() => navigate("/amm")}>
        Go to Trading
      </Button>
    </CardContent>
  </Card>
)}

{/* Replace amount input: */}
<div className="space-y-1">
  <Label htmlFor="deposit_id">Deposit ID</Label>
  <Input
    id="deposit_id"
    value={depositId}
    onChange={(e) => setDepositId(e.target.value)}
    placeholder="deposit-uuid"
  />
</div>
```

> Use `useNavigate` from `react-router-dom`. Verify the exact route path for AMMTradingPage in the bank app's router config.

**`RedeemsPage.tsx`** — auto-inject requester fields, add Zeto toggle:

```typescript
// Same wallet/paladin injection as DepositsPage:
const walletAddress = useAuthStore((state) => state.profile?.wallet_address ?? null);
const paladinIdentity = import.meta.env.VITE_PALADIN_IDENTITY as string;

// Zeto state:
const [zetoToggle, setZetoToggle] = useState(false);
const [zetoHash, setZetoHash] = useState("");

const handleZetoToggle = (checked: boolean) => {
  setZetoToggle(checked);
  if (!checked) setZetoHash("");
};

const zetoValid = !zetoToggle || (zetoHash.startsWith("0x") && zetoHash.length > 2);
const canSubmit = !!walletAddress && !!amount && zetoValid;

// Handler:
const handleRequestRedeem = async (e: FormEvent) => {
  e.preventDefault();
  if (!walletAddress) return;
  await requestRedeem(
    walletAddress,
    paladinIdentity,
    amount,
    zetoToggle ? zetoHash : undefined
  );
};
```

In JSX (add after the amount input):
```tsx
<div className="space-y-2">
  <label className="flex items-center gap-2 text-sm cursor-pointer">
    <input
      type="checkbox"
      checked={zetoToggle}
      onChange={(e) => handleZetoToggle(e.target.checked)}
    />
    My tokens came from a Zeto private transfer
  </label>

  {zetoToggle && (
    <div className="space-y-1 pl-4 border-l-2 border-muted">
      <Label htmlFor="zeto_transfer_tx_hash">Zeto Transfer Tx Hash</Label>
      <Input
        id="zeto_transfer_tx_hash"
        value={zetoHash}
        onChange={(e) => setZetoHash(e.target.value)}
        placeholder="0x..."
        required
      />
      {zetoHash && !zetoHash.startsWith("0x") && (
        <p className="text-xs text-destructive">Must start with 0x</p>
      )}
    </div>
  )}
</div>

{!walletAddress && (
  <p className="text-sm text-destructive">
    Onboarding incomplete — wallet address unavailable.
  </p>
)}

<Button type="submit" disabled={!canSubmit || redeemStatus === "loading"}>
  Request Redeem
</Button>
```

**Verify cleanup**:
```bash
grep -r "CreateAmountRequest" frontend/apps/bank/src/
# Expected: 0 hits
```

---

### T012 — TypeScript + lint verification across both apps (M12)

**Title**: Run `tsc --noEmit` and lint in governance and bank apps; fix all errors introduced by Phase B payload changes

**Description**: Final quality gate for Phase B. Run strict TypeScript typecheck and ESLint in both
apps. Verify the deprecated-field grep commands from the verification plan return zero hits.
Fix any residual errors before closing Phase B.

**Dependencies**: T007, T008, T009, T010, T011

**Acceptance criteria**:
- `pnpm --filter @cbweb3/governance tsc --noEmit` exits with code 0
- `pnpm --filter @cbweb3/governance lint` exits with code 0
- `pnpm --filter @cbweb3/bank tsc --noEmit` exits with code 0
- `pnpm --filter @cbweb3/bank lint` exits with code 0
- All six spot-check grep commands return zero hits
- No `any` implicitly introduced; no `@ts-ignore` added

**Files**:
- Potentially modify any file from T007–T011 to fix type errors

**Implementation hints**:

```bash
# From repo root — run in order:
pnpm --filter @cbweb3/governance tsc --noEmit
pnpm --filter @cbweb3/governance lint

pnpm --filter @cbweb3/bank tsc --noEmit
pnpm --filter @cbweb3/bank lint

# Spot-check deprecated fields (all should return 0 hits):
grep -r "amount_a\|amount_b" \
  frontend/apps/governance/src/types \
  frontend/apps/governance/src/services \
  frontend/apps/governance/src/features/liquidity/liquidity.store.ts

grep -n "approveAmountA\|approveAmountB\|approve_amount_a\|approve_amount_b" \
  frontend/apps/governance/src/features/liquidity/LiquidityManagementPage.tsx

grep -n "mintAmountA\|mintAmountB\|wizard_amount_a\|wizard_amount_b" \
  frontend/apps/governance/src/features/liquidity/CooperativeLiquidityWizard.tsx

grep -r "amount_a\|amount_b" \
  frontend/apps/bank/src/types/amm-v2.types.ts \
  frontend/apps/bank/src/features/amm \
  frontend/apps/bank/src/pages/AMMTradingPage.tsx

grep -r "CreateAmountRequest" frontend/apps/bank/src/

# Optional — Vite production builds:
pnpm --filter @cbweb3/governance build
pnpm --filter @cbweb3/bank build
```

**Common errors and fixes**:

| Error | Root cause | Fix |
|-------|-----------|-----|
| `Property 'amount_a' does not exist on type 'MintAndApproveRequest'` | Caller in T008 or T009 not updated | `grep -rn "amount_a"` → find and update call site |
| `Property 'amount_a' does not exist on type 'ApproveAmmRequest'` | Stale caller in bank app | Check all `approveAmm(` call sites in `AMMTradingPage.tsx` |
| `Expected 1 arguments, but got 2` on `requestEscrow` | Store action changed to 1 arg (deposit_id); page still passes amount | Update call site in `EscrowsPage.tsx` |
| `Property 'wallet_address' does not exist on type 'Profile'` | Field name mismatch | Read `auth.store.ts`; may be `walletAddress` (camelCase) — use actual field name |
| `Argument of type 'string \| null' is not assignable to parameter of type 'string'` | Null wallet_address not guarded | Add `if (!walletAddress) return;` before call |
| `Cannot find name 'ApproveAmmRequest'` | Missing import | Add to import block in `liquidity.api.ts` or `liquidity.store.ts` |
| `Type '"" \| "A" \| "B"' is not assignable to type '"A" \| "B"'` | `approveSide` default `""` passed to `approveAmm` | Add `if (!approveSide) return;` guard or use type assertion after guard |

---

### T013 — Governance payment API: FR-028 route audit

**Title**: Verify governance `payment.api.ts` uses the 7 canonical CB governance payment routes; confirm no stale paths in the codebase; audit `router.go` registration

**Description**: Seven new CB governance payment action routes were registered in `router.go` on
2026-05-20 (branch `feature/frontend-liquidity`) under FR-028. The backend is fully implemented.
This task verifies that the governance frontend's `payment.api.ts` module already calls these
exact canonical paths. If stale paths are found, update `payment.api.ts` accordingly. The
`router.go` audit is read-only and serves as an audit trail record.

The 7 canonical routes are:
- `POST /api/v1/payments/deposits/approve`
- `POST /api/v1/payments/deposits/reject`
- `POST /api/v1/payments/deposits/fiat-exchange`
- `POST /api/v1/payments/escrows/approve`
- `POST /api/v1/payments/escrows/reject`
- `POST /api/v1/payments/redeems/approve`
- `POST /api/v1/payments/redeems/reject`

**Dependencies**: T012 (Phase B complete — ensures no type regressions before route audit)

**Acceptance criteria**:
- `grep -rn "/payment/deposit/register\|/payment/exchange/request\|/payment/redeem/request" frontend/apps/governance/src/` → zero hits
- `frontend/apps/governance/src/services/api/payment.api.ts` contains all 7 canonical paths listed above
- Each of the 7 paths is called via a dedicated exported function (one function per action)
- `grep -n "payments/deposits/approve\|payments/deposits/reject\|payments/deposits/fiat-exchange\|payments/escrows/approve\|payments/escrows/reject\|payments/redeems/approve\|payments/redeems/reject" backend/services/payment-orchestrator/router.go` (or equivalent router file) returns 7 hits — one per route
- No `any` cast introduced; existing TypeScript types used or extended as needed

**Files**:
- Audit (read-only unless stale paths found): `frontend/apps/governance/src/services/api/payment.api.ts`
- Audit (read-only): `backend/services/payment-orchestrator/router.go` (or the router file containing payment routes)
- Potentially modify: `frontend/apps/governance/src/services/api/payment.api.ts` if stale paths are detected

**Implementation hints**:

**Step 1 — Audit governance `payment.api.ts` for stale paths**:

```bash
# Stale path check (all must return 0 hits):
grep -rn "/payment/deposit/register\|/payment/exchange/request\|/payment/redeem/request" \
  frontend/apps/governance/src/

# Canonical path presence check (each must appear):
grep -n "payments/deposits/approve" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/deposits/reject" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/deposits/fiat-exchange" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/escrows/approve" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/escrows/reject" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/redeems/approve" frontend/apps/governance/src/services/api/payment.api.ts
grep -n "payments/redeems/reject" frontend/apps/governance/src/services/api/payment.api.ts
```

**Step 2 — If stale paths are found, update `payment.api.ts`**:

Replace any stale route with the canonical path. Example pattern:

```typescript
// STALE (remove):
approveCBDeposit: async (depositId: string): Promise<void> => {
  await httpClientV1.post(`/payment/deposit/approve/${depositId}`);
},

// CANONICAL (replace with):
approveCBDeposit: async (depositId: string): Promise<void> => {
  await httpClientV1.post(`/payments/deposits/approve`, { deposit_id: depositId });
},
```

The expected function structure covering all 7 actions (adapt to existing naming conventions in the file):

```typescript
// Deposits
approveCBDeposit:      (payload) => httpClientV1.post("/payments/deposits/approve", payload)
rejectCBDeposit:       (payload) => httpClientV1.post("/payments/deposits/reject", payload)
fiatExchangeCBDeposit: (payload) => httpClientV1.post("/payments/deposits/fiat-exchange", payload)

// Escrows
approveCBEscrow:  (payload) => httpClientV1.post("/payments/escrows/approve", payload)
rejectCBEscrow:   (payload) => httpClientV1.post("/payments/escrows/reject", payload)

// Redeems
approveCBRedeem:  (payload) => httpClientV1.post("/payments/redeems/approve", payload)
rejectCBRedeem:   (payload) => httpClientV1.post("/payments/redeems/reject", payload)
```

> The base URL prefix `/api/v1` is typically injected by the `httpClientV1` Axios instance.
> Confirm the base URL in the Axios factory (`frontend/apps/governance/src/lib/http.ts` or
> equivalent) before constructing paths. Only include the path segment after `/api/v1`.

**Step 3 — Audit `router.go` for all 7 routes (read-only)**:

```bash
# Locate the router file:
find backend/ -name "router.go" | head -5

# Confirm all 7 routes are registered:
grep -n "payments/deposits/approve\|payments/deposits/reject\|payments/deposits/fiat-exchange\|payments/escrows/approve\|payments/escrows/reject\|payments/redeems/approve\|payments/redeems/reject" \
  <path-to-router.go>
# Expected: 7 matching lines
```

**Spot-check**:
```bash
# Final stale-path sweep across entire governance app:
grep -rn "/payment/deposit/register\|/payment/exchange/request\|/payment/redeem/request" \
  frontend/apps/governance/src/
# Expected: 0 hits
```

---

## Parallel Execution Examples

### Phase A (COMPLETE — sequential):
```
T001 → T002 → T003 → T004 → T005 → T006
```

### Phase B (two parallel tracks):
```
Track 1 (Governance):  T007 → T008 → T009 ─┐
                                             ├──→ T011 ──→ T012 ──→ T013
Track 2 (Bank AMM):    T010 ────────────────┘
```

T010 can start at the same time as T007 (no shared files). T011 depends on T010
(verify `wallet_address` field name in auth store). T012 must precede T013.
T013 (FR-028 audit) is sequential — runs after T012 confirms TypeScript is clean.

---

## Implementation Strategy

**Phase A (DONE)**: Wizard fully functional with 4 steps, pending-commit banner, TypeScript clean.

**Phase B MVP (T007 + T010)**: Fix the two highest-visibility breakages — governance
MintAndApprove `HTTP 400 DEPRECATED_FIELDS` (T007) and bank AMM approve-amm `HTTP 400 SIDE_REQUIRED`
(T010). After these two tasks, operators can mint/approve in both apps again.

**Phase B complete (T008 + T009 + T011 + T012)**: Full UX updates — single-amount panels,
G5-cross bootstrap block, payment flow updates, Zeto toggle, post-escrow banner. TypeScript
clean across both apps.

**Phase B closure (T013)**: FR-028 route audit — verify governance `payment.api.ts` uses all
7 canonical CB governance payment action routes; remove any stale paths; confirm `router.go`
registration as audit trail. No backend changes required.
