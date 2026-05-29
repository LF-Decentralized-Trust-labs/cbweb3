import axios from "axios";
import type {
  CrossCurrencyQuote,
  CrossCurrencySwapRequest,
  CrossCurrencySwapResult,
} from "../../types/cross-currency-swap.types";
import type { PoolStatus } from "../../types/amm-v2.types";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";
import { apiBaseV2, httpClientV2 } from "./http-client";

export const CROSS_CURRENCY_QUOTE_TTL_MS = 15_000;
export const CROSS_CURRENCY_QUOTE_REFRESH_MS = 10_000;

const crossCurrencyExecuteClient = axios.create({
  baseURL: apiBaseV2,
  withCredentials: true,
  timeout: 300_000,
});

attachAuthInterceptor(crossCurrencyExecuteClient);

export const crossCurrencySwapApi = {
  getQuote: async (
    sourceCurrency: string,
    targetCurrency: string,
    amountOut: string,
  ): Promise<CrossCurrencyQuote> => {
    const response = await httpClientV2.get<CrossCurrencyQuote>("/amm/quote/cross-currency", {
      params: {
        source_currency: sourceCurrency,
        target_currency: targetCurrency,
        amount_out: amountOut,
      },
    });
    return response.data;
  },
  executeSwap: async (payload: CrossCurrencySwapRequest): Promise<CrossCurrencySwapResult> => {
    const response = await crossCurrencyExecuteClient.post<CrossCurrencySwapResult>("/amm/swap/cross-currency", payload);
    return response.data;
  },
  getSwapStatus: async (swapId: string): Promise<CrossCurrencySwapResult> => {
    const response = await httpClientV2.get<CrossCurrencySwapResult>(`/amm/swap/cross-currency/${swapId}`);
    return response.data;
  },
  getPoolStatus: async (pair: string): Promise<PoolStatus> => {
    const response = await httpClientV2.get<PoolStatus>(`/amm/pool/${pair}/status`);
    return response.data;
  },
};

export function calcMaxAmountIn(amountInWei: string, slippageBps = 1100): string {
  return ((BigInt(amountInWei) * BigInt(slippageBps)) / 1000n).toString();
}

export function weiToDisplay(wei: string, decimals = 6): string {
  if (!wei || !/^\d+$/.test(wei)) {
    return "—";
  }

  const whole = BigInt(wei) / 10n ** 18n;
  const frac = (BigInt(wei) % 10n ** 18n)
    .toString()
    .padStart(18, "0")
    .slice(0, decimals);

  return `${whole}.${frac}`;
}
