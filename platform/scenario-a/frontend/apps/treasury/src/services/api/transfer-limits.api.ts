// SPDX-License-Identifier: Apache-2.0

import type { CreateTransferLimitRequest, ListTransferLimitsResponse, TransferLimit } from "../../types";
import { httpClient } from "./http-client";

export const transferLimitsApi = {
  list: async (centralBankId?: string): Promise<TransferLimit[]> => {
    const params = centralBankId ? { central_bank_id: centralBankId } : {};
    const response = await httpClient.get<ListTransferLimitsResponse>("/treasury/transfer-limits", { params });
    return response.data.limits ?? [];
  },

  create: async (req: CreateTransferLimitRequest): Promise<TransferLimit> => {
    const response = await httpClient.post<TransferLimit>("/treasury/transfer-limits", req);
    return response.data;
  },

  delete: async (limitId: string): Promise<void> => {
    await httpClient.delete(`/treasury/transfer-limits/${encodeURIComponent(limitId)}`);
  },
};
