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
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { useCrossCurrencySwapStore } from "../features/amm/cross-currency-swap.store";
import { useAmmV2Store } from "../features/amm/amm-v2.store";
import { usePolling } from "../hooks/usePolling";
import { QUOTE_REFRESH_INTERVAL_MS, QUOTE_STALE_AFTER_MS } from "../services/api/amm-v2.api";
import {
  calcMaxAmountIn,
  CROSS_CURRENCY_QUOTE_REFRESH_MS,
  weiToDisplay,
} from "../services/api/cross-currency-swap.api";
import { useAuthStore } from "../stores";
import { SWAP_ERROR } from "../types/amm-v2.types";
import { CROSS_CURRENCY_SWAP_ERROR } from "../types/cross-currency-swap.types";

export function AMMTradingPage() {
  return <AMMTradingV2 />;
}

function AMMTradingV2() {
  const quote = useAmmV2Store((state) => state.quote);
  const quoteTimestamp = useAmmV2Store((state) => state.quoteTimestamp);
  const swapResult = useAmmV2Store((state) => state.swapResult);
  const poolStatus = useAmmV2Store((state) => state.poolStatus);
  const circuitBreakerState = useAmmV2Store((state) => state.circuitBreakerState);
  const status = useAmmV2Store((state) => state.status);
  const error = useAmmV2Store((state) => state.error);
  const ammApproved = useAmmV2Store((state) => state.ammApproved);
  const fetchQuote = useAmmV2Store((state) => state.fetchQuote);
  const executeSwap = useAmmV2Store((state) => state.executeSwap);
  const fetchPoolStatus = useAmmV2Store((state) => state.fetchPoolStatus);
  const fetchCircuitBreakerState = useAmmV2Store((state) => state.fetchCircuitBreakerState);
  const approveAmm = useAmmV2Store((state) => state.approveAmm);
  const sessionBankId = useAuthStore((state) => state.profile?.bankId ?? "");

  const [pair, setPair] = useState("BRL-ARS");
  const [amountOut, setAmountOut] = useState("");
  const [maxAmountIn, setMaxAmountIn] = useState("");
  const [beneficiaryId, setBeneficiaryId] = useState("bank-b");

  const [approveAmount, setApproveAmount] = useState("");
  const [showApprovePanel, setShowApprovePanel] = useState(false);
  const [nowTimestamp, setNowTimestamp] = useState(() => Date.now());

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNowTimestamp(Date.now());
    }, 1000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, []);

  const isQuoteStale = useMemo(() => {
    if (!quoteTimestamp) {
      return false;
    }
    return nowTimestamp - quoteTimestamp > QUOTE_STALE_AFTER_MS;
  }, [quoteTimestamp, nowTimestamp]);

  const isHalted = circuitBreakerState === "HALTED";
  const isPoolInactive = !!poolStatus?.pool_status && poolStatus.pool_status !== "ACTIVE";

  useEffect(() => {
    void fetchPoolStatus(pair);
  }, [fetchPoolStatus, pair]);

  usePolling(
    () => {
      void fetchCircuitBreakerState(pair);
    },
    15000,
    true,
  );

  usePolling(
    () => {
      if (!amountOut) {
        return;
      }
      void fetchQuote(pair, amountOut);
    },
    QUOTE_REFRESH_INTERVAL_MS,
    Boolean(quote && amountOut),
  );

  const handleGetQuote = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await fetchQuote(pair, amountOut);
  };

  const handleSwap = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (isHalted || isPoolInactive) {
      return;
    }

    await executeSwap({
      pair,
      amount_out: amountOut,
      max_amount_in: maxAmountIn,
      payer_id: sessionBankId,
      beneficiary_id: beneficiaryId,
    });
  };

  const handleApproveAmm = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await approveAmm(approveAmount);
  };

  const swapErrorMessage = useMemo(() => {
    if (error === SWAP_ERROR.SLIPPAGE_LIMIT_EXCEEDED) {
      return "Market moved — retry with updated quote";
    }
    if (error === SWAP_ERROR.POOL_NOT_ACTIVE) {
      return "Pool is not ACTIVE yet. Wait for governance activation before swapping.";
    }
    if (error === SWAP_ERROR.INSUFFICIENT_POOL_LIQUIDITY) {
      return "Insufficient pool liquidity — contact Central Bank";
    }
    if (error === SWAP_ERROR.ZK_VALIDATION_FAILED) {
      return "ZK validation failed — check ZK pointer field";
    }
    if (error === SWAP_ERROR.CIRCUIT_BREAKER_HALTED) {
      return "Swaps temporarily suspended by Central Bank";
    }
    if (error === SWAP_ERROR.APPROVE_AMM_REQUIRED) {
      return "Approve AMM first, then submit swap.";
    }
    return error;
  }, [error]);

  return (
    <Tabs defaultValue="exact-output" className="space-y-4">
      <TabsList>
        <TabsTrigger value="exact-output">Exact Output</TabsTrigger>
        <TabsTrigger value="cross-currency">Cross-Currency</TabsTrigger>
      </TabsList>

      <TabsContent value="exact-output" className="space-y-4">
        {isHalted ? (
          <div className="rounded-md border border-destructive/40 bg-destructive/10 p-3">
            <p className="text-sm font-semibold text-destructive">Circuit Breaker HALTED</p>
            <p className="text-sm text-destructive">Swaps temporarily suspended by Central Bank.</p>
          </div>
        ) : null}

        {poolStatus?.imbalance_flag ? (
          <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
            <p className="text-sm font-semibold text-amber-700">Pool Imbalance</p>
            <p className="text-sm text-amber-700">Imbalance flag is active for this pair.</p>
          </div>
        ) : null}

        <div className="grid gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>Quote Exact Output</CardTitle>
              <CardDescription>Get required input and price impact for BRL-ARS output amount.</CardDescription>
            </CardHeader>
            <CardContent>
              <form className="space-y-3" onSubmit={handleGetQuote}>
                <div className="space-y-1">
                  <Label htmlFor="pair">Pair</Label>
                  <Input id="pair" value={pair} onChange={(event) => setPair(event.target.value)} required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="amount_out">Amount Out</Label>
                  <Input id="amount_out" value={amountOut} onChange={(event) => setAmountOut(event.target.value)} inputMode="numeric" pattern="[0-9]+" required />
                </div>
                <Button type="submit" disabled={status === "loading"}>
                  {status === "loading" ? "Submitting..." : "Get Quote"}
                </Button>
              </form>

              {quote ? (
                <div className="mt-4 space-y-1 rounded-md border border-border p-3 text-sm">
                  <p>Required Input: {quote.required_input}</p>
                  <p>Price Impact: {(parseFloat(quote.price_impact) * 100).toFixed(2)}%</p>
                  <p>Quote Timestamp: {new Date(quote.quote_timestamp * 1000).toLocaleString()}</p>
                  {isQuoteStale ? (
                    <div className="flex items-center gap-2">
                      <Badge variant="warning">Quote stale (&gt;10s)</Badge>
                      <Button type="button" variant="outline" size="sm" onClick={() => void fetchQuote(pair, amountOut)}>
                        Refresh Quote
                      </Button>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Swap Exact Output</CardTitle>
              <CardDescription>Execute swap with slippage control.</CardDescription>
            </CardHeader>
            <CardContent>
              <form className="space-y-3" onSubmit={handleSwap}>
                <div className="space-y-1">
                  <Label htmlFor="swap_pair">Pair</Label>
                  <Input id="swap_pair" value={pair} onChange={(event) => setPair(event.target.value)} required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="swap_amount_out">Amount Out</Label>
                  <Input id="swap_amount_out" value={amountOut} onChange={(event) => setAmountOut(event.target.value)} inputMode="numeric" pattern="[0-9]+" required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="max_amount_in">Max Amount In</Label>
                  <Input id="max_amount_in" value={maxAmountIn} onChange={(event) => setMaxAmountIn(event.target.value)} inputMode="numeric" pattern="[0-9]+" required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="payer_id">Payer ID</Label>
                  <Input id="payer_id" value={sessionBankId} readOnly required />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="beneficiary_id">Beneficiary ID</Label>
                  <Input id="beneficiary_id" value={beneficiaryId} onChange={(event) => setBeneficiaryId(event.target.value)} required />
                </div>
                {isPoolInactive ? (
                  <p className="text-sm text-destructive">
                    Swaps are disabled until pool status is ACTIVE. Current status: {poolStatus?.pool_status}
                  </p>
                ) : null}
                {!ammApproved ? (
                  <p className="text-sm text-muted-foreground">Complete Approve AMM before submitting swap.</p>
                ) : null}
                <Button type="submit" disabled={status === "loading" || isHalted || isPoolInactive || !ammApproved}>
                  {status === "loading" ? "Submitting..." : "Execute Swap"}
                </Button>
              </form>

              {swapResult ? (
                <div className="mt-4 space-y-1 rounded-md border border-border p-3 text-sm">
                  <p>Order ID: {swapResult.order_id}</p>
                  <p>Tx Hash: {swapResult.tx_hash}</p>
                  <p>Amount In: {swapResult.amount_in}</p>
                  <p>Confirmed At: {swapResult.confirmed_at}</p>
                </div>
              ) : null}

              {swapErrorMessage ? (
                <div className="mt-3 space-y-2">
                  <p className="text-sm text-destructive">{swapErrorMessage}</p>
                  {error === SWAP_ERROR.SLIPPAGE_LIMIT_EXCEEDED ? (
                    <Button type="button" variant="outline" size="sm" onClick={() => void fetchQuote(pair, amountOut)}>
                      Refresh Quote
                    </Button>
                  ) : null}
                </div>
              ) : null}
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Pool Status</CardTitle>
          </CardHeader>
          <CardContent className="space-y-1 text-sm">
            <p>Pair: {poolStatus?.pool_pair ?? pair}</p>
            <p>Reserve A: {poolStatus?.reserve_a ?? "-"}</p>
            <p>Reserve B: {poolStatus?.reserve_b ?? "-"}</p>
            <p>Current Ratio: {poolStatus?.current_ratio ?? "-"}</p>
            <p>Updated At: {poolStatus?.updated_at ?? "-"}</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Approve AMM</CardTitle>
            <CardDescription>Optional token approval helper for Scenario B liquidity workflow.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Button type="button" variant="secondary" onClick={() => setShowApprovePanel((value) => !value)}>
              {showApprovePanel ? "Hide Approve Panel" : "Show Approve Panel"}
            </Button>
            {showApprovePanel ? (
              <form className="space-y-3" onSubmit={handleApproveAmm}>
                <div className="space-y-1">
                  <Label htmlFor="approve_amount">Amount</Label>
                  <Input id="approve_amount" value={approveAmount} onChange={(event) => setApproveAmount(event.target.value)} inputMode="numeric" pattern="[0-9]+" required />
                </div>
                <Button type="submit" variant="outline" disabled={status === "loading"}>
                  {status === "loading" ? "Submitting..." : "Submit Approve AMM"}
                </Button>
              </form>
            ) : null}
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="cross-currency" className="space-y-4">
        <CrossCurrencySwapPanel />
      </TabsContent>
    </Tabs>
  );
}

const CROSS_CURRENCY_POOL_PAIR = "W-BRL-ARS";

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
};

function CrossCurrencySwapPanel() {
  const poolStatus = useCrossCurrencySwapStore((state) => state.poolStatus);
  const quote = useCrossCurrencySwapStore((state) => state.quote);
  const quoteExpiresAt = useCrossCurrencySwapStore((state) => state.quoteExpiresAt);
  const swapResult = useCrossCurrencySwapStore((state) => state.swapResult);
  const step = useCrossCurrencySwapStore((state) => state.step);
  const error = useCrossCurrencySwapStore((state) => state.error);
  const errorCode = useCrossCurrencySwapStore((state) => state.errorCode);
  const bridgeOutAcknowledged = useCrossCurrencySwapStore((state) => state.bridgeOutAcknowledged);
  const quoteWasRefreshed = useCrossCurrencySwapStore((state) => state.quoteWasRefreshed);
  const fetchPoolStatus = useCrossCurrencySwapStore((state) => state.fetchPoolStatus);
  const fetchQuote = useCrossCurrencySwapStore((state) => state.fetchQuote);
  const executeSwap = useCrossCurrencySwapStore((state) => state.executeSwap);
  const acknowledgeSwapError = useCrossCurrencySwapStore((state) => state.acknowledgeSwapError);
  const clearQuoteRefreshedNotice = useCrossCurrencySwapStore((state) => state.clearQuoteRefreshedNotice);
  const reset = useCrossCurrencySwapStore((state) => state.reset);

  const [sourceCurrency, setSourceCurrency] = useState("BRL");
  const [targetCurrency, setTargetCurrency] = useState("ARS");
  const [amountOut, setAmountOut] = useState("");
  const [beneficiaryBankId, setBeneficiaryBankId] = useState("");
  const [nowTimestamp, setNowTimestamp] = useState(() => Date.now());

  useEffect(() => {
    void fetchPoolStatus(CROSS_CURRENCY_POOL_PAIR);
  }, [fetchPoolStatus]);

  usePolling(
    () => {
      void fetchPoolStatus(CROSS_CURRENCY_POOL_PAIR);
    },
    30_000,
    true,
  );

  usePolling(
    () => {
      if (!amountOut) {
        return;
      }
      void fetchQuote(sourceCurrency, targetCurrency, amountOut);
    },
    CROSS_CURRENCY_QUOTE_REFRESH_MS,
    step === "idle" && quote !== null && amountOut.length > 0,
  );

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNowTimestamp(Date.now());
    }, 1000);

    return () => {
      window.clearInterval(intervalId);
    };
  }, []);

  useEffect(() => {
    if (!amountOut) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      void fetchQuote(sourceCurrency, targetCurrency, amountOut);
    }, 600);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [amountOut, fetchQuote, sourceCurrency, targetCurrency]);

  const ttlSeconds = useMemo(() => {
    if (!quoteExpiresAt) {
      return null;
    }

    return Math.max(0, Math.ceil((quoteExpiresAt - nowTimestamp) / 1000));
  }, [nowTimestamp, quoteExpiresAt]);

  const poolLifecycleStatus = poolStatus?.pool_status ?? "UNKNOWN";
  const isPoolActive = poolLifecycleStatus === "ACTIVE";
  const isQuoteExpired = quoteExpiresAt !== null && nowTimestamp > quoteExpiresAt;
  const maxAmountIn = quote ? calcMaxAmountIn(quote.amount_in) : "";

  const isExecuteDisabled =
    step !== "idle" ||
    !isPoolActive ||
    quote === null ||
    isQuoteExpired;

  const handleGetQuote = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    clearQuoteRefreshedNotice();
    await fetchQuote(sourceCurrency, targetCurrency, amountOut);
  };

  const handleExecuteSwap = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!quote) {
      return;
    }

    clearQuoteRefreshedNotice();
    await executeSwap({
      source_currency: sourceCurrency,
      target_currency: targetCurrency,
      pool_pair: CROSS_CURRENCY_POOL_PAIR,
      amount_out: amountOut,
      max_amount_in: calcMaxAmountIn(quote.amount_in),
      beneficiary_bank_id: beneficiaryBankId,
      quote_id: quote.quote_id,
    });
  };

  const progressSteps = ["BRIDGE_IN_PROGRESS", "SWAP_IN_PROGRESS", "BRIDGE_OUT_PROGRESS", "COMPLETED"];
  const activeProgressIndex = swapResult?.status ? progressSteps.indexOf(swapResult.status) : -1;

  const defaultErrorMessage =
    errorCode && CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
      ? CROSS_CURRENCY_ERROR_MESSAGES[errorCode]
      : "An unexpected error occurred.";

  return (
    <div className="space-y-4">
      {isPoolActive ? (
        <div className="rounded-md border border-emerald-500/40 bg-emerald-500/10 p-3">
          <p className="text-sm font-semibold text-emerald-700">Pool W-BRL-ARS - ACTIVE</p>
        </div>
      ) : (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
          <p className="text-sm font-semibold text-amber-700">Pool W-BRL-ARS is currently {poolLifecycleStatus}. Contact your Central Bank for status.</p>
        </div>
      )}

      {quoteWasRefreshed && step === "idle" ? (
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
          <p className="text-sm text-amber-700">Quote was refreshed. Review and confirm again.</p>
        </div>
      ) : null}

      {step === "idle" ? (
        <Card>
          <CardHeader>
            <CardTitle>Cross-Currency Quote</CardTitle>
            <CardDescription>Request BRL to ARS quote for pair W-BRL-ARS.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleGetQuote}>
              <div className="space-y-1">
                <Label htmlFor="cross_source_currency">Source Currency</Label>
                <Input
                  id="cross_source_currency"
                  value={sourceCurrency}
                  onChange={(event) => {
                    clearQuoteRefreshedNotice();
                    setSourceCurrency(event.target.value);
                  }}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="cross_target_currency">Target Currency</Label>
                <Input
                  id="cross_target_currency"
                  value={targetCurrency}
                  onChange={(event) => {
                    clearQuoteRefreshedNotice();
                    setTargetCurrency(event.target.value);
                  }}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="cross_amount_out">Amount Out (wei)</Label>
                <Input
                  id="cross_amount_out"
                  value={amountOut}
                  onChange={(event) => {
                    clearQuoteRefreshedNotice();
                    setAmountOut(event.target.value);
                  }}
                  inputMode="numeric"
                  pattern="[0-9]+"
                  required
                />
              </div>
              <Button type="submit" disabled={!isPoolActive}>
                Get Quote
              </Button>
            </form>
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
            <p>
              Amount In: {weiToDisplay(quote.amount_in)} {sourceCurrency}
            </p>
            <p>
              Amount Out: {weiToDisplay(quote.amount_out)} {targetCurrency}
            </p>
            <p>TTL: {ttlSeconds ?? "-"}s</p>
          </CardContent>
        </Card>
      ) : null}

      {quote && step === "idle" ? (
        <Card>
          <CardHeader>
            <CardTitle>Execute Cross-Currency Swap</CardTitle>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleExecuteSwap}>
              <div className="space-y-1">
                <Label htmlFor="cross_beneficiary_bank_id">Beneficiary Bank ID</Label>
                <Input id="cross_beneficiary_bank_id" value={beneficiaryBankId} onChange={(event) => setBeneficiaryBankId(event.target.value)} required />
              </div>
              <div className="space-y-1">
                <Label>Max Amount In (with slippage)</Label>
                <p className="rounded-md border border-border p-2 text-sm">
                  {weiToDisplay(maxAmountIn)} {sourceCurrency}
                </p>
              </div>
              <Button
                type="submit"
                disabled={isExecuteDisabled}
                title={isQuoteExpired ? "Quote expired - click Get Quote to refresh" : undefined}
              >
                Execute Swap
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {(step === "submitting" || step === "polling") ? (
        <Card>
          <CardHeader>
            <CardTitle>Swap Progress</CardTitle>
          </CardHeader>
          <CardContent>
            <ol className="grid gap-2 md:grid-cols-4">
              {progressSteps.map((progressStep, index) => {
                const isCurrent = swapResult?.status === progressStep;
                const isCompleted = activeProgressIndex >= 0 && index < activeProgressIndex;
                return (
                  <li key={progressStep} className="rounded-md border border-border p-2 text-sm">
                    <Badge variant={isCurrent ? "default" : isCompleted ? "success" : "outline"}>
                      {index + 1}
                    </Badge>{" "}
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
            <CardTitle>Swap Completed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>Swap Tx Hash: {swapResult.swap_tx_hash ?? "-"}</p>
            <p>Amount In: {weiToDisplay(swapResult.amount_in ?? "")}</p>
            <p>Amount Out: {weiToDisplay(swapResult.amount_out ?? "")}</p>
            <p>Bridge In Position: {swapResult.bridge_in_position_id ?? "-"}</p>
            <p>Bridge Out Position: {swapResult.bridge_out_position_id ?? "-"}</p>
            <p>Correlation ID: {swapResult.correlation_id ?? "-"}</p>
            <Button type="button" variant="outline" onClick={reset}>
              Reset
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {step === "failed" && errorCode !== CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED ? (
        <Card>
          <CardHeader>
            <CardTitle>Swap Error</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <p>{defaultErrorMessage}</p>
            {!errorCode || !CROSS_CURRENCY_ERROR_MESSAGES[errorCode] ? (
              <details>
                <summary>Technical details</summary>
                <p>Error Code: {errorCode ?? "unknown"}</p>
                <p>Error: {error ?? "n/a"}</p>
              </details>
            ) : null}
            <Button type="button" variant="outline" onClick={reset}>
              Try Again
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {step === "failed" && errorCode === CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED ? (
        <Card className="border-destructive/60 bg-destructive/5">
          <CardHeader>
            <CardTitle className="text-destructive">Critical: Partial Swap - Manual Action Required</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <p>
              Source funds were already debited. Manual reconciliation is required - contact your Central Bank with Swap ID:
              <strong> {swapResult?.swap_id ?? "unknown"}</strong>.
            </p>
            <div className="flex gap-2">
              <Button type="button" variant="destructive" onClick={acknowledgeSwapError}>
                Acknowledge
              </Button>
              {bridgeOutAcknowledged ? (
                <Button type="button" variant="outline" onClick={reset}>
                  Reset Form
                </Button>
              ) : null}
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
