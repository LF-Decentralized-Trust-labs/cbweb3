# Research: 010-cross-currency-swap-frontend

## Summary of Resolved Questions

All clarifications were resolved in the spec's Session 2026-05-27 block. No open unknowns remain.

---

## Decision 1 — Store isolation

**Decision**: Dedicated `useCrossCurrencySwapStore` — `amm-v2.store.ts` is untouched.

**Rationale**: Single-purpose stores, no regression risk on existing exact-output flow, clean state isolation between tabs.

**Alternatives considered**: Extending `useAmmV2Store` with new state fields → rejected (increases coupling, risks breaking existing swap flow on shared `status` field).

---

## Decision 2 — Axios timeout isolation

**Decision**: `cross-currency-swap.api.ts` creates a dedicated axios instance with `timeout: 60_000` **only** for `executeSwap`. All other methods (`getPoolStatus`, `getQuote`, `getSwapStatus`) use `httpClientV2` directly.

**Rationale**: POST `/amm/swap/cross-currency` is a synchronous 10–30s orchestrator call. Applying 60s to `httpClientV2` would affect every other endpoint globally.

**Implementation**: Import `apiBaseV2` constant derivation from `http-client.ts` (or re-derive using the same `VITE_API_URL` env var), create a new `axios.create({ baseURL: apiBaseV2, withCredentials: true, timeout: 60_000 })` instance, and call `attachAuthInterceptor` on it.

**Alternatives considered**: Per-call `timeout` override on `httpClientV2` via config parameter → viable but less explicit; per-instance is clearer for future maintainers.

---

## Decision 3 — Dual-path executeSwap (sync COMPLETED vs polling)

**Decision**: If `POST /amm/swap/cross-currency` returns `status === "COMPLETED"`, transition directly to `step: completed`. Otherwise call `pollSwapStatus(swap_id)` at 2s intervals for up to 90 attempts.

**Rationale**: Backend spec for 009 says the POST can return COMPLETED inline if all three steps finish within the request window. The fallback polling covers network cuts or intermediate responses.

**Alternatives considered**: Always poll after POST → unnecessary round-trips when result is already terminal.

---

## Decision 4 — Quote auto-refresh strategy

**Decision**: `usePolling` hook fires `fetchQuote` every 10s while `step === idle` AND a quote exists. Auto-refresh is suspended as soon as step leaves `idle`. Manual [Get Quote] remains available at any time.

**Rationale**: `usePolling` is already present in the codebase with `enabled` guard — trivial to reuse. The `enabled` boolean is `step === "idle" && Boolean(quote && amountOut)`.

**Alternatives considered**: `setInterval` inside the component → equivalent but duplicates the `usePolling` abstraction.

---

## Decision 5 — BigInt-only arithmetic

**Decision**: `calcMaxAmountIn` and `weiToDisplay` MUST use `BigInt` throughout. No `parseFloat`, no `Number()` on wei strings.

**Rationale**: 18-decimal wei values exceed `Number.MAX_SAFE_INTEGER` for large amounts. `parseFloat` would silently corrupt precision.

**calcMaxAmountIn signature**:
```typescript
export function calcMaxAmountIn(amountInWei: string, slippageBps: number = 1100): string {
  return (BigInt(amountInWei) * BigInt(slippageBps) / 1000n).toString();
}
```

**weiToDisplay signature** (returns up to 6 decimal places, returns `"—"` on non-numeric input):
```typescript
export function weiToDisplay(wei: string, decimals: number = 6): string {
  if (!wei || !/^\d+$/.test(wei)) return "—";
  const divisor = 10n ** 18n;
  const whole = BigInt(wei) / divisor;
  const remainder = BigInt(wei) % divisor;
  const fracStr = remainder.toString().padStart(18, "0").slice(0, decimals);
  return `${whole.toString()}.${fracStr}`;
}
```

---

## Decision 6 — BRIDGE_OUT_FAILED critical alert

**Decision**: `BRIDGE_OUT_FAILED` renders a **critical persistent alert** (visually distinct, red border/background, not dismissable by `reset()` alone). Requires explicit operator acknowledgement before form resets. The `reset()` action is NOT called automatically.

**Rationale**: Funds are debited on the Hub side — operator MUST contact Central Bank. Silent dismissal would hide a reconciliation obligation.

**UI pattern**: A boolean `acknowledged` local state gates whether `reset()` can be called.

---

## Decision 7 — Tab structure in AMMTradingV2

**Decision**: Wrap the existing AMMTradingV2 content in a top-level `<Tabs>` component with two values: `"exact-output"` (existing cards) and `"cross-currency"` (new feature). `<Tabs defaultValue="exact-output">` preserves current default behaviour.

**Rationale**: AMMTradingV2 currently renders a flat grid, not a tabbed layout. The spec requires coexistence as a tab. Wrapping in Tabs is the minimal change — existing cards become a single `TabsContent` value without modification.

**Risk**: None — `@cbweb3/ui` Tabs are already imported in this file.

---

## Decision 8 — PoolStatus type reuse

**Decision**: `crossCurrencySwapApi.getPoolStatus` returns `PoolStatus` from `amm-v2.types.ts` — no duplicate type defined.

**Rationale**: The backend returns the same pool status shape for both the exact-output pool endpoint and the cross-currency pair. Reusing avoids drift.

---

## Technology findings

| Item | Finding |
|------|---------|
| `usePolling` signature | `(callback, intervalMs, enabled, maxDurationMs?)` — `enabled` boolean controls start/stop |
| `attachAuthInterceptor` | Accepts `AxiosInstance`, must be called on the new dedicated execute instance |
| `@cbweb3/ui` Tabs | Already imported in `AMMTradingPage.tsx` — no new import needed |
| `httpClientV2` timeout | Currently **not set** (confirmed by reading `http-client.ts`) |
| Test location convention | `__tests__/` subdirectory inside `features/amm/` and `services/api/` |
| TypeScript strict mode | Confirmed — `no any` rule applies to all new files |
