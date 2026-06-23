// SPDX-License-Identifier: Apache-2.0

import type {
  AMMQuote,
  ApproveAmmRequest,
  PoolStatus,
  SwapOrder,
  SwapRequest,
} from "../../types/amm-v2.types";
import { httpClientV2 } from "./http-client";

export const QUOTE_REFRESH_INTERVAL_MS = 12_000;
export const QUOTE_STALE_AFTER_MS = 10_000;

export const ammV2Api = {
  getQuote: async (pair: string, amount_out: string): Promise<AMMQuote> => {
    const response = await httpClientV2.get<AMMQuote>("/amm/quote/exact-output", {
      params: { pair, amount_out },
    });
    return response.data;
  },
  executeSwap: async (payload: SwapRequest): Promise<SwapOrder> => {
    const response = await httpClientV2.post<SwapOrder>("/amm/swap/exact-output", payload);
    return response.data;
  },
  getPoolStatus: async (pair: string): Promise<PoolStatus> => {
    const response = await httpClientV2.get<PoolStatus>(`/amm/pool/${pair}/status`);
    return response.data;
  },
  approveAmm: async (payload: ApproveAmmRequest): Promise<{ status: string }> => {
    const response = await httpClientV2.post<{ status: string }>("/amm/token/approve-amm", payload);
    return response.data;
  },
};
