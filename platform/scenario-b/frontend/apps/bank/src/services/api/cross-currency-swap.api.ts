// SPDX-License-Identifier: Apache-2.0

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
    poolPair?: string,
  ): Promise<CrossCurrencyQuote> => {
    const response = await httpClientV2.get<CrossCurrencyQuote>("/amm/quote/cross-currency", {
      params: {
        source_currency: sourceCurrency,
        target_currency: targetCurrency,
        amount_out: amountOut,
        // Pass the exact pair_id so the quote resolves the right pool (pair ids do
        // not always follow the derived "W-{source}-W-{target}" convention).
        ...(poolPair ? { pool_pair: poolPair } : {}),
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

// weiToDisplay converts a raw base-unit string to a human-readable decimal string.
// tokenDecimals is the number of decimal places the token uses (read from contract).
// displayDecimals controls how many fractional digits to show (default 6).
export function weiToDisplay(wei: string, tokenDecimals: number, displayDecimals = 6): string {
  if (!wei || !/^\d+$/.test(wei)) {
    return "—";
  }

  const divisor = 10n ** BigInt(tokenDecimals);
  const whole = BigInt(wei) / divisor;
  const frac = (BigInt(wei) % divisor)
    .toString()
    .padStart(tokenDecimals, "0")
    .slice(0, displayDecimals);

  return frac.replace(/0+$/, "") ? `${whole}.${frac.replace(/0+$/, "")}` : whole.toString();
}
