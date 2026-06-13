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
import {
  calcMaxAmountIn,
  CROSS_CURRENCY_QUOTE_REFRESH_MS,
  weiToDisplay,
} from "../services/api/cross-currency-swap.api";
import { usePaymentStore } from "../stores";
import { CROSS_CURRENCY_SWAP_ERROR } from "../types/cross-currency-swap.types";
import { displayToBase, formatAmountInput, parseAmountInput } from "../types";

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
  [CROSS_CURRENCY_SWAP_ERROR.TRANSFER_LIMIT_EXCEEDED]:
    "Daily transfer limit reached. Your Central Bank has set a maximum transfer volume for today. Contact your Central Bank to adjust the limit.",
};

export function TransferPage() {
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
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const tokenDecimals = tCeBMDecimals ?? 18;

  const [sourceCurrency, setSourceCurrency] = useState("BRL");
  const [targetCurrency, setTargetCurrency] = useState("ARS");
  const [amountOut, setAmountOut] = useState("");
  const [amountInInput, setAmountInInput] = useState("");
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
      void fetchQuote(sourceCurrency, targetCurrency, displayToBase(parseAmountInput(amountOut), tokenDecimals));
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

  const suggestedMaxAmountIn = quote ? calcMaxAmountIn(quote.amount_in) : "";

  const ttlSeconds = useMemo(() => {
    if (!quoteExpiresAt) {
      return null;
    }

    return Math.max(0, Math.ceil((quoteExpiresAt - nowTimestamp) / 1000));
  }, [nowTimestamp, quoteExpiresAt]);

  const poolLifecycleStatus = poolStatus?.pool_status ?? "UNKNOWN";
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
    await fetchQuote(sourceCurrency, targetCurrency, displayToBase(parseAmountInput(amountOut), tokenDecimals));
  };

  const handleExecuteTransfer = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!quote) {
      return;
    }

    clearQuoteRefreshedNotice();
    await executeSwap({
      source_currency: sourceCurrency,
      target_currency: targetCurrency,
      pool_pair: CROSS_CURRENCY_POOL_PAIR,
      amount_out: displayToBase(parseAmountInput(amountOut), tokenDecimals),
      max_amount_in: displayToBase(parseAmountInput(amountInInput), tokenDecimals),
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
      <Card>
        <CardHeader>
          <CardTitle>Transfer</CardTitle>
          <CardDescription>
            Get a BRL-ARS quote and execute cross-currency transfer. Deposit, escrow, AMM approval and redeem are handled on dedicated pages.
          </CardDescription>
        </CardHeader>
      </Card>

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

      <Card>
        <CardHeader>
          <CardTitle>Step 1: Get Quote</CardTitle>
          <CardDescription>Request quote for the transfer amount.</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-3" onSubmit={handleGetQuote}>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <Label htmlFor="transfer-source-currency">Source Currency</Label>
                <Input
                  id="transfer-source-currency"
                  value={sourceCurrency}
                  onChange={(event) => {
                    clearQuoteRefreshedNotice();
                    setSourceCurrency(event.target.value);
                  }}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="transfer-target-currency">Target Currency</Label>
                <Input
                  id="transfer-target-currency"
                  value={targetCurrency}
                  onChange={(event) => {
                    clearQuoteRefreshedNotice();
                    setTargetCurrency(event.target.value);
                  }}
                  required
                />
              </div>
            </div>
            <div className="space-y-1">
              <Label htmlFor="transfer-amount-out">Amount Out</Label>
              <Input
                id="transfer-amount-out"
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
            <Button type="submit" disabled={!isPoolActive || step !== "idle"}>Get Quote</Button>
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
            <CardTitle>Step 2: Execute Transfer</CardTitle>
            <CardDescription>Provide max input amount and beneficiary, then confirm execution.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleExecuteTransfer}>
              <div className="space-y-1">
                <Label htmlFor="transfer-max-amount-in">Max Amount In</Label>
                <Input
                  id="transfer-max-amount-in"
                  value={amountInInput}
                  onChange={(event) => {
                    const cleaned = event.target.value.replace(/[^0-9.]/g, "").replace(/(\..*)\./g, "$1");
                    setAmountInInput(formatAmountInput(cleaned));
                  }}
                  inputMode="decimal"
                  placeholder="0.00"
                  required
                />
                <p className="text-xs text-muted-foreground">Suggested max from quote + slippage: {weiToDisplay(suggestedMaxAmountIn, tokenDecimals)} {sourceCurrency}</p>
                <Button type="button" variant="outline" size="sm" onClick={() => setAmountInInput(formatAmountInput(weiToDisplay(suggestedMaxAmountIn, tokenDecimals)))}>
                  Use Suggested Max
                </Button>
              </div>
              <div className="space-y-1">
                <Label htmlFor="transfer-beneficiary-bank-id">Beneficiary Bank ID</Label>
                <Input
                  id="transfer-beneficiary-bank-id"
                  value={beneficiaryBankId}
                  onChange={(event) => setBeneficiaryBankId(event.target.value)}
                  required
                />
              </div>
              <Button
                type="submit"
                disabled={isExecuteDisabled}
                title={isQuoteExpired ? "Quote expired - click Get Quote to refresh" : undefined}
              >
                Execute Transfer
              </Button>
            </form>
          </CardContent>
        </Card>
      ) : null}

      {(step === "submitting" || step === "polling") ? (
        <Card>
          <CardHeader>
            <CardTitle>Transfer Progress</CardTitle>
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
            <CardTitle>Transfer Completed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <p>Swap Tx Hash: {swapResult.swap_tx_hash ?? "-"}</p>
            <p>Amount In: {weiToDisplay(swapResult.amount_in ?? "", tokenDecimals)}</p>
            <p>Amount Out: {weiToDisplay(swapResult.amount_out ?? "", tokenDecimals)}</p>
            <p>Bridge In Position: {swapResult.bridge_in_position_id ?? "-"}</p>
            <p>Bridge Out Position: {swapResult.bridge_out_position_id ?? "-"}</p>
            <p>Correlation ID: {swapResult.correlation_id ?? "-"}</p>
            <Button type="button" variant="outline" onClick={reset}>Reset</Button>
          </CardContent>
        </Card>
      ) : null}

      {step === "failed" && errorCode !== CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED ? (
        <Card>
          <CardHeader>
            <CardTitle>Transfer Error</CardTitle>
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
            <Button type="button" variant="outline" onClick={reset}>Try Again</Button>
          </CardContent>
        </Card>
      ) : null}

      {step === "failed" && errorCode === CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED ? (
        <Card className="border-destructive/60 bg-destructive/5">
          <CardHeader>
            <CardTitle className="text-destructive">Critical: Partial Transfer - Manual Action Required</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <p>
              Source funds were already debited. Manual reconciliation is required - contact your Central Bank with Swap ID:
              <strong> {swapResult?.swap_id ?? "unknown"}</strong>.
            </p>
            <div className="flex gap-2">
              <Button type="button" variant="destructive" onClick={acknowledgeSwapError}>Acknowledge</Button>
              {bridgeOutAcknowledged ? (
                <Button type="button" variant="outline" onClick={reset}>Reset Form</Button>
              ) : null}
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
