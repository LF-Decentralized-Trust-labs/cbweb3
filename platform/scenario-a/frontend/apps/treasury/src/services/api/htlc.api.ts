// SPDX-License-Identifier: Apache-2.0

import type { HTLCLock, SearchHTLCParams, SearchHTLCResponse } from "../../types";
import { httpClient } from "./http-client";

export const htlcApi = {
  search: async (params: SearchHTLCParams): Promise<SearchHTLCResponse> => {
    const response = await httpClient.get<SearchHTLCResponse>("/htlc/search", { params });
    return response.data;
  },
  getStatus: async (contractId: string): Promise<HTLCLock> => {
    const response = await httpClient.get<HTLCLock>(`/htlc/status/${encodeURIComponent(contractId)}`);
    return response.data;
  },
};
