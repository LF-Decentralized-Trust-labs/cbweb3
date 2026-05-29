import axios from "axios";
import { create } from "zustand";
import {
  CROSS_CURRENCY_QUOTE_TTL_MS,
  crossCurrencySwapApi,
} from "../../services/api/cross-currency-swap.api";
import {
  CROSS_CURRENCY_SWAP_ERROR,
  type CrossCurrencyQuote,
  type CrossCurrencySwapErrorCode,
  type CrossCurrencySwapRequest,
  type CrossCurrencySwapResult,
} from "../../types/cross-currency-swap.types";
import type { PoolStatus } from "../../types/amm-v2.types";

const POLL_INTERVAL_MS = 2_000;
const POLL_MAX_ATTEMPTS = 90;

const knownErrorCodes = new Set<string>(Object.values(CROSS_CURRENCY_SWAP_ERROR));

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

type SwapStep = "idle" | "submitting" | "polling" | "completed" | "failed";

interface CrossCurrencySwapState {
  poolStatus: PoolStatus | null;
  quote: CrossCurrencyQuote | null;
  quoteExpiresAt: number | null;
  swapResult: CrossCurrencySwapResult | null;
  step: SwapStep;
  error: string | null;
  errorCode: CrossCurrencySwapErrorCode | null;
  bridgeOutAcknowledged: boolean;
  quoteWasRefreshed: boolean;
}

interface CrossCurrencySwapActions {
  fetchPoolStatus: (pair: string) => Promise<void>;
  fetchQuote: (sourceCurrency: string, targetCurrency: string, amountOut: string) => Promise<void>;
  executeSwap: (req: CrossCurrencySwapRequest) => Promise<void>;
  pollSwapStatus: (swapId: string) => Promise<void>;
  acknowledgeSwapError: () => void;
  clearQuoteRefreshedNotice: () => void;
  reset: () => void;
}

type CrossCurrencySwapStore = CrossCurrencySwapState & CrossCurrencySwapActions;

function toErrorCode(value?: string | null): CrossCurrencySwapErrorCode | null {
  if (!value || !knownErrorCodes.has(value)) {
    return null;
  }

  return value as CrossCurrencySwapErrorCode;
}

function parseApiError(error: unknown): { code: string | null; message: string | null; swapId: string | null } {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as
      | {
          error?: string;
          error_code?: string;
          message?: string;
          swap_id?: string;
        }
      | undefined;

    return {
      code: payload?.error_code ?? payload?.error ?? null,
      message: payload?.message ?? payload?.error ?? error.message,
      swapId: payload?.swap_id ?? null,
    };
  }

  if (error instanceof Error) {
    return { code: null, message: error.message, swapId: null };
  }

  return { code: null, message: "Unexpected swap error.", swapId: null };
}

const initialState: CrossCurrencySwapState = {
  poolStatus: null,
  quote: null,
  quoteExpiresAt: null,
  swapResult: null,
  step: "idle",
  error: null,
  errorCode: null,
  bridgeOutAcknowledged: false,
  quoteWasRefreshed: false,
};

export const useCrossCurrencySwapStore = create<CrossCurrencySwapStore>((set, get) => ({
  ...initialState,
  fetchPoolStatus: async (pair) => {
    try {
      const poolStatus = await crossCurrencySwapApi.getPoolStatus(pair);
      set({ poolStatus, error: null });
    } catch (error) {
      const parsed = parseApiError(error);
      set({ error: parsed.message ?? "Unable to fetch pool status." });
    }
  },
  fetchQuote: async (sourceCurrency, targetCurrency, amountOut) => {
    try {
      const quote = await crossCurrencySwapApi.getQuote(sourceCurrency, targetCurrency, amountOut);
      set({
        quote,
        quoteExpiresAt: Date.now() + CROSS_CURRENCY_QUOTE_TTL_MS,
        error: null,
        errorCode: null,
      });
    } catch (error) {
      const parsed = parseApiError(error);
      set({
        quote: null,
        quoteExpiresAt: null,
        error: parsed.message ?? "Unable to fetch quote.",
      });
    }
  },
  executeSwap: async (req) => {
    set({ step: "submitting", error: null, errorCode: null, quoteWasRefreshed: false });

    let requestPayload = { ...req };
    const quoteExpiresAt = get().quoteExpiresAt;
    if (quoteExpiresAt !== null && Date.now() > quoteExpiresAt) {
      await get().fetchQuote(req.source_currency, req.target_currency, req.amount_out);
      const refreshedQuote = get().quote;
      if (!refreshedQuote) {
        set({
          step: "failed",
          errorCode: CROSS_CURRENCY_SWAP_ERROR.QUOTE_EXPIRED,
          error: "Quote expired and could not be refreshed.",
        });
        return;
      }

      requestPayload = { ...requestPayload, quote_id: refreshedQuote.quote_id };
    }

    const submitSwap = async (payload: CrossCurrencySwapRequest, allowRetryOnQuoteExpired: boolean): Promise<void> => {
      try {
        const result = await crossCurrencySwapApi.executeSwap(payload);

        if (result.status === "COMPLETED") {
          set({ swapResult: result, step: "completed", error: null, errorCode: null });
          return;
        }

        if (result.swap_id) {
          set({
            swapResult: result,
            error: null,
            errorCode: null,
          });
          await get().pollSwapStatus(result.swap_id);
          return;
        }

        set({
          step: "failed",
          error: "Swap execution returned no operation ID.",
          errorCode: null,
        });
      } catch (error) {
        const parsed = parseApiError(error);

        if (parsed.code === CROSS_CURRENCY_SWAP_ERROR.QUOTE_EXPIRED && allowRetryOnQuoteExpired) {
          await get().fetchQuote(payload.source_currency, payload.target_currency, payload.amount_out);
          const refreshedQuote = get().quote;
          if (!refreshedQuote) {
            set({
              step: "failed",
              errorCode: CROSS_CURRENCY_SWAP_ERROR.QUOTE_EXPIRED,
              error: "Quote expired and could not be refreshed.",
            });
            return;
          }

          set({ quoteWasRefreshed: true });
          await submitSwap({ ...payload, quote_id: refreshedQuote.quote_id }, false);
          return;
        }

        if (parsed.code === CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED) {
          const swapId = parsed.swapId ?? get().swapResult?.swap_id;
          set({
            step: "failed",
            errorCode: CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED,
            bridgeOutAcknowledged: false,
            swapResult: swapId
              ? {
                  ...(get().swapResult ?? { swap_id: swapId, status: "FAILED" }),
                  swap_id: swapId,
                  status: "FAILED",
                  error_code: CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED,
                }
              : get().swapResult,
            error: parsed.message ?? "Bridge-out failed and requires manual reconciliation.",
          });
          return;
        }

        set({
          step: "failed",
          errorCode: toErrorCode(parsed.code),
          error: parsed.message ?? "Swap execution failed.",
        });
      }
    };

    await submitSwap(requestPayload, true);
  },
  pollSwapStatus: async (swapId) => {
    for (let attempt = 0; attempt < POLL_MAX_ATTEMPTS; attempt += 1) {
      set({ step: "polling" });
      await wait(POLL_INTERVAL_MS);

      try {
        const result = await crossCurrencySwapApi.getSwapStatus(swapId);
        set({ swapResult: result });

        if (result.status === "COMPLETED") {
          set({ step: "completed", error: null, errorCode: null });
          return;
        }

        if (result.status === "FAILED") {
          set({
            step: "failed",
            errorCode: toErrorCode(result.error_code),
            error: result.error_code ?? "Swap failed.",
          });
          return;
        }
      } catch (error) {
        const parsed = parseApiError(error);
        if (parsed.code === CROSS_CURRENCY_SWAP_ERROR.SWAP_NOT_FOUND || (axios.isAxiosError(error) && error.response?.status === 404)) {
          set({
            step: "failed",
            errorCode: CROSS_CURRENCY_SWAP_ERROR.SWAP_NOT_FOUND,
            error: "Swap operation could not be found.",
          });
          return;
        }
      }
    }

    set({
      step: "failed",
      errorCode: null,
      error: "Swap status unknown after 3 minutes.",
    });
  },
  acknowledgeSwapError: () => {
    set({ bridgeOutAcknowledged: true });
  },
  clearQuoteRefreshedNotice: () => {
    set({ quoteWasRefreshed: false });
  },
  reset: () => {
    const state = get();
    if (
      state.errorCode === CROSS_CURRENCY_SWAP_ERROR.BRIDGE_OUT_FAILED
      && state.bridgeOutAcknowledged === false
    ) {
      return;
    }

    set((current) => ({
      ...initialState,
      poolStatus: current.poolStatus,
    }));
  },
}));
