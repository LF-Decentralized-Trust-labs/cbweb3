import type { CreateTransferLimitPayload, TransferLimit } from "../../types";
import { httpClient } from "./http-client";

const BASE = "/api/v2/governance/transfer-limits";

export const transferLimitsApi = {
  list: async (): Promise<TransferLimit[]> => {
    const res = await httpClient.get<{ limits: TransferLimit[] }>(BASE, {
      baseURL: import.meta.env.VITE_API_BASE_URL?.replace("/api/v1", "") ?? "",
    });
    return res.data.limits;
  },

  create: async (payload: CreateTransferLimitPayload): Promise<TransferLimit> => {
    const res = await httpClient.post<TransferLimit>(BASE, payload, {
      baseURL: import.meta.env.VITE_API_BASE_URL?.replace("/api/v1", "") ?? "",
    });
    return res.data;
  },

  remove: async (limitId: string): Promise<void> => {
    await httpClient.delete(`${BASE}/${limitId}`, {
      baseURL: import.meta.env.VITE_API_BASE_URL?.replace("/api/v1", "") ?? "",
    });
  },
};
