// SPDX-License-Identifier: Apache-2.0

import type {
  HTLCLock,
  LockHTLCRequest,
  LockHTLCResponse,
  LockWithHashHTLCRequest,
  RefundHTLCResponse,
  SearchHTLCParams,
  SearchHTLCResponse,
  SettleHTLCRequest,
  SettleHTLCResponse,
} from "../../types";
import { httpClient } from "./http-client";

export const htlcApi = {
  lock: async (payload: LockHTLCRequest): Promise<LockHTLCResponse> => {
    const response = await httpClient.post<LockHTLCResponse>("/htlc/lock", payload);
    return response.data;
  },
  lockWithHash: async (payload: LockWithHashHTLCRequest): Promise<LockHTLCResponse> => {
    const response = await httpClient.post<LockHTLCResponse>("/htlc/lock-with-hash", payload);
    return response.data;
  },
  getStatus: async (contractId: string): Promise<HTLCLock> => {
    const response = await httpClient.get<HTLCLock>(`/htlc/status/${encodeURIComponent(contractId)}`);
    return response.data;
  },
  settle: async (payload: SettleHTLCRequest): Promise<SettleHTLCResponse> => {
    const response = await httpClient.post<SettleHTLCResponse>("/htlc/settle", payload);
    return response.data;
  },
  refund: async (contractId: string): Promise<RefundHTLCResponse> => {
    const response = await httpClient.post<RefundHTLCResponse>("/htlc/refund", {
      contract_id: contractId,
    });
    return response.data;
  },
  search: async (params: SearchHTLCParams): Promise<SearchHTLCResponse> => {
    const response = await httpClient.get<SearchHTLCResponse>("/htlc/search", { params });
    return response.data;
  },
};
