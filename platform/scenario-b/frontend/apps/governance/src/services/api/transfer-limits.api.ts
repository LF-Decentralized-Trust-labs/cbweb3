// SPDX-License-Identifier: Apache-2.0

import type { CreateTransferLimitPayload, TransferLimit } from "../../types";
import { httpClientV2 } from "./http-client";

const BASE = "/governance/transfer-limits";

export const transferLimitsApi = {
  list: async (): Promise<TransferLimit[]> => {
    const res = await httpClientV2.get<{ limits: TransferLimit[] }>(BASE);
    return res.data.limits;
  },

  create: async (payload: CreateTransferLimitPayload): Promise<TransferLimit> => {
    const res = await httpClientV2.post<TransferLimit>(BASE, payload);
    return res.data;
  },

  remove: async (limitId: string): Promise<void> => {
    await httpClientV2.delete(`${BASE}/${limitId}`);
  },
};
