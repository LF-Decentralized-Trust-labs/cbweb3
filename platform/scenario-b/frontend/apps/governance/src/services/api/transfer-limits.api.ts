// SPDX-License-Identifier: Apache-2.0

import type { CreateTransferLimitPayload, TransferLimit } from "../../types";
import { httpClientV2 } from "./http-client";

const BASE = "/governance/transfer-limits";

export const transferLimitsApi = {
  // Returns the CB's limits plus its sovereign currency (the spoke's own currency),
  // so the UI can show the currency read-only instead of accepting free text.
  list: async (): Promise<{ limits: TransferLimit[]; sovereignCurrency: string }> => {
    const res = await httpClientV2.get<{ limits: TransferLimit[]; sovereign_currency?: string }>(BASE);
    return { limits: res.data.limits ?? [], sovereignCurrency: res.data.sovereign_currency ?? "" };
  },

  create: async (payload: CreateTransferLimitPayload): Promise<TransferLimit> => {
    const res = await httpClientV2.post<TransferLimit>(BASE, payload);
    return res.data;
  },

  remove: async (limitId: string): Promise<void> => {
    await httpClientV2.delete(`${BASE}/${limitId}`);
  },
};
