import type {
  AcceptRejectFXResponse,
  FXAgreement,
  ProposeFXAgreementRequest,
  ProposeFXAgreementResponse,
} from "../../types";
import { httpClient } from "./http-client";

export const fxAgreementApi = {
  propose: async (payload: ProposeFXAgreementRequest): Promise<ProposeFXAgreementResponse> => {
    const response = await httpClient.post<ProposeFXAgreementResponse>("/payments/fx/agreements", payload);
    return response.data;
  },
  accept: async (tradeId: string): Promise<AcceptRejectFXResponse> => {
    const response = await httpClient.post<AcceptRejectFXResponse>(
      `/payments/fx/agreements/${encodeURIComponent(tradeId)}/accept`,
    );
    return response.data;
  },
  reject: async (tradeId: string): Promise<AcceptRejectFXResponse> => {
    const response = await httpClient.post<AcceptRejectFXResponse>(
      `/payments/fx/agreements/${encodeURIComponent(tradeId)}/reject`,
    );
    return response.data;
  },
  cancel: async (tradeId: string): Promise<AcceptRejectFXResponse> => {
    const response = await httpClient.post<AcceptRejectFXResponse>(
      `/payments/fx/agreements/${encodeURIComponent(tradeId)}/cancel`,
    );
    return response.data;
  },
  settle: async (tradeId: string): Promise<AcceptRejectFXResponse> => {
    const response = await httpClient.post<AcceptRejectFXResponse>(
      `/payments/fx/agreements/${encodeURIComponent(tradeId)}/settle`,
    );
    return response.data;
  },
  get: async (tradeId: string): Promise<FXAgreement> => {
    const response = await httpClient.get<{ agreement: FXAgreement }>(
      `/payments/fx/agreements/${encodeURIComponent(tradeId)}`,
    );
    return response.data.agreement;
  },
  list: async (params?: { counterparty?: string; state?: string }): Promise<FXAgreement[]> => {
    const response = await httpClient.get<{ agreements: FXAgreement[] }>("/payments/fx/agreements", { params });
    return response.data.agreements || [];
  },
};
