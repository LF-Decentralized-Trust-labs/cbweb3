import type { FundingDecisionPayload, FundingRequest } from "../../types";
import { httpClient } from "./http-client";

type FundingRequestsResponse = {
  requests: FundingRequest[];
};

export const fundingApi = {
  list: async (): Promise<FundingRequest[]> => {
    const response = await httpClient.get<FundingRequestsResponse>("/funding-requests");
    return response.data.requests ?? [];
  },
  approve: async (payload: FundingDecisionPayload): Promise<FundingRequest> => {
    const response = await httpClient.post<FundingRequest>("/funding-requests/approve", payload);
    return response.data;
  },
  reject: async (payload: FundingDecisionPayload): Promise<FundingRequest> => {
    const response = await httpClient.post<FundingRequest>("/funding-requests/reject", payload);
    return response.data;
  },
};
