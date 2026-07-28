// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
} from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { useCrossCurrencySwapStore } from "../features/amm/cross-currency-swap.store";
import { usePolling } from "../hooks/usePolling";
import { ammV2Api } from "../services/api/amm-v2.api";
import {
  calcMaxAmountIn,
  CROSS_CURRENCY_QUOTE_REFRESH_MS,
  weiToDisplay,
} from "../services/api/cross-currency-swap.api";
import { usePaymentStore } from "../stores";
import type { HubCurrency, HubPair } from "../types/amm-v2.types";
import { CROSS_CURRENCY_SWAP_ERROR } from "../types/cross-currency-swap.types";
import { displayToBase, formatAmountInput, parseAmountInput } from "../types";

const CROSS_CURRENCY_ERROR_MESSAGES: Record<string, string> = {
  [CROSS_CURRENCY_SWAP_ERROR.CIRCUIT_BREAKER_HALTED]:
    "Pool is temporarily paused by the Central Bank. Please try again later or contact your Central Bank.",
  [CROSS_CURRENCY_SWAP_ERROR.SLIPPAGE_LIMIT_EXCEEDED]:
    "Price moved beyond your slippage tolerance. Please refresh the quote and try again.",
  [CROSS_CURRENCY_SWAP_ERROR.BRIDGE_IN_FAILED]:
    "Could not lock your funds on the source network. The operation has been rolled back. No funds were lost.",
  [CROSS_CURRENCY_SWAP_ERROR.INSUFFICIENT_POOL_LIQUIDITY]:
    "Not enough liquidity in the pool for this amount. Try a smaller amount or wait for the Central Bank to add liquidity.",
  [CROSS_CURRENCY_SWAP_ERROR.QUOTE_EXPIRED]:
    "The quote expired during submission. A new quote has been fetched - please review and confirm again.",
  [CROSS_CURRENCY_SWAP_ERROR.SWAP_FAILED]:
    "The swap transaction failed on the Hub. No funds were moved. Please try again.",
  [CROSS_CURRENCY_SWAP_ERROR.SWAP_NOT_FOUND]:
    "The swap operation could not be found. Please contact support with the operation reference.",
  [CROSS_CURRENCY_SWAP_ERROR.INVALID_REQUEST]:
    "The request was invalid. Please verify your inputs and try again.",
  [CROSS_CURRENCY_SWAP_ERROR.TRANSFER_LIMIT_EXCEEDED]:
    "Daily transfer limit reached. Your Central Bank has set a maximum transfer volume for today. Contact your Central Bank to adjust the limit.",
};

// currencyCode strips the sovereign W-token prefix down to the base ISO code:
// "W-tCeBM_BRL" -> "BRL". The pair_id itself is NOT split (its hub token symbols
// contain dashes), so codes are resolved from token addresses via /hub/currencies.
function currencyCode(symbol: string): string {
  const parts = symbol.split("_");
  return parts[parts.length - 1] || symbol;
}

export function CrossCurrencyBridgePage() {
  const poolStatus = useCrossCurrencySwapStore((state) => state.poolStatus);
  const quote = useCrossCurrencySwapStore((state) => state.quote);
  const quoteExpiresAt = useCrossCurrencySwapStore((state) => state.quoteExpiresAt);
  const swapResult = useCrossCurrencySwapStore((state) => state.swapResult);
  const step = useCrossCurrencySwapStore((state) => state.step);
  const error = useCrossCurrencySwapStore((state) => state.error);
  const errorCode = useCrossCurrencySwapStore((state) => state.errorCode);
  const quoteWasRefreshed = useCrossCurrencySwapStore((state) => state.quoteWasRefreshed);
  const fetchPoolStatus = useCrossCurrencySwapStore((state) => state.fetchPoolStatus);
  const fetchQuote = useCrossCurrencySwapStore((state) => state.fetchQuote);
  const executeSwap = useCrossCurrencySwapStore((state) => state.executeSwap);
  const clearQuoteRefreshedNotice = useCrossCurrencySwapStore((state) => state.clearQuoteRefreshedNotice);
  const reset = useCrossCurrencySwapStore((state) => state.reset);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tCeBMSymbol = usePaymentStore((state) => state.tCeBMSymbol);
  const fetchBalances = usePaymentStore((state) => state.fetchAll);
  const tokenDecimals = tCeBMDecimals ?? 18;

  const [pairs, setPairs] = useState<HubPair[]>([]);
  const [currencies, setCurrencies] = useState<HubCurrency[]>([]);
  const [poolsError, setPoolsError] = useState<string | null>(null);
  const [selectedPairId, setSelectedPairId] = useState("");
  const [amountOut, setAmountOut] = useState("");
  const [amountInInput, setAmountInInput] = useState("");
  const [beneficiaryBankId, setBeneficiaryBankId] = useState("");
  const [nowTimestamp, setNowTimestamp] = useState(() => Date.now());

  // Load the CB-built pools + the currency registry + this bank's own token symbol.
  useEffect(() => {
    reset();
    if (!tCeBMSymbol) {
      void fetchBalances();
    }
    void (async () => {
      try {
        const [allPairs, allCurrencies] = await Promise.all([ammV2Api.getPairs(), ammV2Api.getCurrencies()]);
        setPairs(allPairs.filter((p) => p.status === "ACTIVE"));
        setCurrencies(allCurrencies);
      } catch {
        setPoolsError("Unable to load liquidity pools. Contact your Central Bank.");
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const addressToCode = useMemo(() => {
    const map = new Map<string, string>();
    for (const c of currencies) {
      map.set(c.token_address.toLowerCase(), currencyCode(c.symbol));
    }
    return map;
  }, [currencies]);

  const codeOf = (addr: string) => addressToCode.get(addr.toLowerCase()) ?? "";

  // The bank's sovereign currency = the code of its own spoke tCeBM. Token symbols
  // vary by deployment ("tBRL", "tCeBM_BRL", "W-tCeBM_BRL"), so resolve it by matching
  // the symbol's suffix against the currency codes actually registered on the hub.
  const knownCodes = useMemo(
    () => Array.from(new Set(currencies.map((c) => currencyCode(c.symbol)))),
    [currencies],
  );
  const homeCurrency = useMemo(() => {
    const sym = (tCeBMSymbol ?? "").toUpperCase();
    return knownCodes.find((code) => code && sym.endsWith(code.toUpperCase())) ?? "";
  }, [knownCodes, tCeBMSymbol]);

  // Sovereignty filter: only pools that include this bank's own currency are
  // swappable from its spoke (e.g. Brazil sees BRL⇄ARS, BRL⇄COP — never ARS⇄COP).
  const swappablePairs = useMemo(
    () =>
      homeCurrency
        ? pairs.filter((p) => codeOf(p.token_a_address) === homeCurrency || codeOf(p.token_b_address) === homeCurrency)
        : [],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [pairs, addressToCode, homeCurrency],
  );

  // Auto-select the first eligible pool once currencies + home currency resolve.
  useEffect(() => {
    if (swappablePairs.length > 0 && !swappablePairs.some((p) => p.pair_id === selectedPairId)) {
      setSelectedPairId(swappablePairs[0].pair_id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [swappablePairs]);

  const selectedPair = useMemo(
    () => swappablePairs.find((p) => p.pair_id === selectedPairId) ?? null,
    [swappablePairs, selectedPairId],
  );

  // Direction is NOT user-selectable: the bank always SPENDS its sovereign
  // currency (source); the beneficiary receives the pair's counterpart (target).
  const codeA = selectedPair ? codeOf(selectedPair.token_a_address) : "";
  const codeB = selectedPair ? codeOf(selectedPair.token_b_address) : "";
  const sourceCurrency = homeCurrency;
  const targetCurrency = selectedPair ? (codeA === homeCurrency ? codeB : codeA) : "";

  // Pool status follows the selected pool; changing pool clears the prior quote.
  useEffect(() => {
    if (!selectedPairId) {
      return;
    }
    reset();
    setAmountOut("");
    setAmountInInput("");
    void fetchPoolStatus(selectedPairId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedPairId]);

  usePolling(
    () => {
      if (selectedPairId) {
        void fetchPoolStatus(selectedPairId);
      }
    },
    30_000,
    Boolean(selectedPairId),
  );

  usePolling(
    () => {
      if (!amountOut || !sourceCurrency || !targetCurrency) {
        return;
      }
      void fetchQuote(sourceCurrency, targetCurrency, displayToBase(parseAmountInput(amountOut), tokenDecimals), selectedPair?.pair_id);
    },
    CROSS_CURRENCY_QUOTE_REFRESH_MS,
    step === "idle" && quote !== null && amountOut.length > 0,
  );

  useEffect(() => {
    const intervalId = window.setInterval(() => setNowTimestamp(Date.now()), 1000);
    return () => window.clearInterval(intervalId);
  }, []);

  const suggestedMaxAmountIn = quote ? calcMaxAmountIn(quote.amount_in) : "";

  const ttlSeconds = useMemo(() => {
    if (!quoteExpiresAt) {
      return null;
    }
    return Math.max(0, Math.ceil((quoteExpiresAt - nowTimestamp) / 1000));
  }, [nowTimestamp, quoteExpiresAt]);

  const poolLifecycleStatus = poolStatus?.pool_status ?? (selectedPair ? "UNKNOWN" : "NO_POOL");
  const isPoolActive = poolLifecycleStatus === "ACTIVE";
  const isQuoteExpired = quoteExpiresAt !== null && nowTimestamp > quoteExpiresAt;

  const isExecuteDisabled =
    step !== "idle"
    || !isPoolActive
    || quote === null
    || !amountOut.trim()
    || !amountInInput.trim()
    || !beneficiaryBankId.trim()
    || isQuoteExpired;

  const handleGetQuote = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    clearQuoteRefreshedNotice();
    await fetchQuote(sourceCurrency, targetCurrency, displayToBase(parseAmountInput(amountOut), tokenDecimals), selectedPair?.pair_id);
  };

  const handleExecuteSwap = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!quote || !selectedPair) {
      return;
    }
    clearQuoteRefreshedNotice();
    await executeSwap({
      source_currency: sourceCurrency,
      target_currency: targetCurrency,
      pool_pair: selectedPair.pair_id,
      amount_out: displayToBase(parseAmountInput(amountOut), tokenDecimals),
      max_amount_in: displayToBase(parseAmountInput(amountInInput), tokenDecimals),
      beneficiary_bank_id: beneficiaryBankId,
      quote_id: quote.quote_id,
    });
  };

  const progressSteps = ["BRIDGE_IN_PROGRESS", "SWAP_IN_PROGRESS", "BRIDGE_OUT_PROGRESS", "COMPLETED"];
  const activeProgressIndex = swapResult?.status ? progressSteps.indexOf(swapResult.status) : -1;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Bridge</CardTitle>
          <CardDescription>
            Cross-currency bridge over a liquidity pool provisioned by the Central Banks. Select a pool, get a
            quote, and execute. Requires tokenised reserves (see Reserve Tokenisation).
          </CardDescription>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Step 1: Select Pool</CardTitle>
          <CardDescription>
            Corridors that include your sovereign currency{homeCurrency ? ` (${homeCurrency})` : ""}. You always
            spend {homeCurrency || "your currency"}; the beneficiary receives the counterpart.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {poolsError ? <p className="text-sm text-destructive">{poolsError}</p> : null}
          {!poolsError && !homeCurrency ? (
            <p className="text-sm text-muted-foreground">Resolving your sovereign currency…</p>
          ) : null}
          {!poolsError && homeCurrency && swappablePairs.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No active pools include {homeCurrency}. Ask your Central Bank to open a {homeCurrency} corridor and
              seed liquidity.
            </p>
          ) : null}
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-1">
              <Label htmlFor="swap-pool">Liquidity Pool</Label>
              <select
                id="swap-pool"
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                value={selectedPairId}
                onChange={(event) => setSelectedPairId(event.target.value)}
                disabled={swappablePairs.length === 0}
              >
                <option value="" disabled>
                  Select a pool…
                </option>
                {swappablePairs.map((p) => {
                  const a = codeOf(p.token_a_address) || "A";
                  const b = codeOf(p.token_b_address) || "B";
                  return (
                    <option key={p.pair_id} value={p.pair_id}>
                      {a} ⇄ {b} — {p.pair_id}
                    </option>
                  );
                })}
              </select>
            </div>
            <div className="space-y-1">
              <Label>Direction (fixed by sovereignty)</Label>
              <div className="flex h-9 items-center">
                <span className="rounded-md border border-border px-2 py-1 text-sm font-medium">
                  {sourceCurrency || "-"} → {targetCurrency || "-"}
                </span>
              </div>
              <p className="text-xs text-muted-foreground">
                You spend {sourceCurrency || "-"} (your sovereign currency); beneficiary receives{" "}
                {targetCurrency || "-"}.
              </p>
            </div>
          </div>
          {selectedPair ? (
            isPoolActive ? (
              <div className="rounded-md border border-emerald-500/40 bg-emerald-500/10 p-3">
                <p className="text-sm font-semibold text-emerald-700">
                  Pool {selectedPair.pair_id} — ACTIVE
                </p>
                <p className="text-xs text-muted-foreground">
                  Reserves: {weiToDisplay(poolStatus?.reserve_a ?? "", tokenDecimals)} {codeA} /{" "}
                  {weiToDisplay(poolStatus?.reserve_b ?? "", tokenDecimals)} {codeB}
                </p>
              </div>
            ) : (
              <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
                <p className="text-sm font-semibold text-amber-700">
                  Pool {selectedPair.pair_id} is {poolLifecycleStatus}. Contact your Central Bank for status.
                </p>
              </div>
            )
          ) : null}
        </CardContent>
      </Card>

      {quoteWasRefreshed && step === "idle" ? (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
          <p className="text-sm text-amber-700">Quote was refreshed. Review and confirm again.</p>
        </div>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Step 2: Get Quote</CardTitle>
          <CardDescription>
            Amount the beneficiary receives in {targetCurrency || "target currency"} (exact output).
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-3" onSubmit={handleGetQuote}>
            <div className="space-y-1">
              <Label htmlFor="swap-amount-out">Amount Out ({targetCurrency || "-"})</Label>
              <Input
                id="swap-amount-out"
                value={amountOut}
                onChange={(event) => {
                  clearQuoteRefreshedNotice();
                  const cleaned = event.target.value.replace(/[^0-9.]/g, "").replace(/(\..*)\./g, "$1");
                  setAmountOut(formatAmountInput(cleaned));
                }}
                inputMode="decimal"
                placeholder="0.00"
                required
              />
            </div>
            <Button type="submit" disabled={!isPoolActive || step !== "idle" || !amountOut.trim()}>
              Get Quote
            </Button>
          </form>
        </CardContent>
      </Card>

      {step === "idle" && error && !quote ? (
        <Card>
          <CardHeader>
            <CardTitle>Quote Error</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p className="text-destructive">
              {errorCode && CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
                ? CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
                : error}
            </p>
          </CardContent>
        </Card>
      ) : null}

      {quote ? (
        <Card>
          <CardHeader>
            <CardTitle>Quote Details</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>Effective Rate: {quote.effective_rate}</p>
            <p>Amount In: {weiToDisplay(quote.amount_in, tokenDecimals)} {sourceCurrency}</p>
            <p>Amount Out: {weiToDisplay(quote.amount_out, tokenDecimals)} {targetCurrency}</p>
            <p>TTL: {ttlSeconds ?? "-"}s</p>
          </CardContent>
        </Card>
      ) : null}

      {quote ? (
        <Card>
          <CardHeader>
            <CardTitle>Step 3: Execute Bridge</CardTitle>
            <CardDescription>Provide max input amount and beneficiary bank, then confirm.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleExecuteSwap}>
              <div className="space-y-1">
                <Label htmlFor="swap-max-amount-in">Max Amount In ({sourceCurrency || "-"})</Label>
                <Input
                  id="swap-max-amount-in"
                  value={amountInInput}
                  onChange={(event) => {
                    const cleaned = event.target.value.replace(/[^0-9.]/g, "").replace(/(\..*)\./g, "$1");
                    setAmountInInput(formatAmountInput(cleaned));
                  }}
                  inputMode="decimal"
                  placeholder="0.00"
                  required
                />
                <p className="text-xs text-muted-foreground">
                  Suggested max from quote + slippage: {weiToDisplay(suggestedMaxAmountIn, tokenDecimals)}{" "}
                  {sourceCurrency}
                </p>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setAmountInInput(formatAmountInput(weiToDisplay(suggestedMaxAmountIn, tokenDecimals)))}
                >
                  Use Suggested Max
                </Button>
              </div>
              <div className="space-y-1">
                <Label htmlFor="swap-beneficiary-bank-id">Beneficiary Bank ID</Label>
                <Input
                  id="swap-beneficiary-bank-id"
                  value={beneficiaryBankId}
                  onChange={(event) => setBeneficiaryBankId(event.target.value)}
                  placeholder="e.g. bank-macro"
                  required
                />
              </div>
              <Button
                type="submit"
                disabled={isExecuteDisabled}
                title={isQuoteExpired ? "Quote expired - click Get Quote to refresh" : undefined}
              >
                Execute Bridge
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {step === "submitting" || step === "polling" ? (
        <Card>
          <CardHeader>
            <CardTitle>Bridge Progress</CardTitle>
          </CardHeader>
          <CardContent>
            <ol className="grid gap-2 md:grid-cols-4">
              {progressSteps.map((progressStep, index) => {
                const isCurrent = swapResult?.status === progressStep;
                const isCompleted = activeProgressIndex >= 0 && index < activeProgressIndex;
                return (
                  <li key={progressStep} className="rounded-md border border-border p-2 text-sm">
                    <Badge variant={isCurrent ? "default" : isCompleted ? "success" : "outline"}>{index + 1}</Badge>{" "}
                    {progressStep.replaceAll("_", " ")}
                  </li>
                );
              })}
            </ol>
          </CardContent>
        </Card>
      ) : null}

      {step === "completed" && swapResult ? (
        <Card>
          <CardHeader>
            <CardTitle>Bridge Completed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>Bridge Tx Hash: {swapResult.swap_tx_hash ?? "-"}</p>
            <p>Amount In: {weiToDisplay(swapResult.amount_in ?? "", tokenDecimals)} {sourceCurrency}</p>
            <p>Amount Out: {weiToDisplay(swapResult.amount_out ?? "", tokenDecimals)} {targetCurrency}</p>
            <p>Correlation ID: {swapResult.correlation_id ?? "-"}</p>
            <Button type="button" variant="outline" onClick={reset}>
              New Bridge
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {step === "failed" ? (
        <Card>
          <CardHeader>
            <CardTitle>Bridge Failed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p className="text-destructive">
              {errorCode && CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
                ? CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
                : error ?? "An unexpected error occurred."}
            </p>
            <Button type="button" variant="outline" onClick={reset}>
              Try Again
            </Button>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
