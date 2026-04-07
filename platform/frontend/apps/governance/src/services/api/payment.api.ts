import type {
  ApproveDepositRequest,
  ApproveEscrowRequest,
  ApproveEscrowResponse,
  ApproveRedeemRequest,
  ApproveRedeemResponse,
  FiatExchangeRequest,
  FiatExchangeResponse,
  ListDepositsResponse,
  ListEscrowsResponse,
  ListRedeemsResponse,
  RejectDepositRequest,
  RejectEscrowRequest,
  RejectRedeemRequest,
} from "../../types";
import { httpClient } from "./http-client";

export const paymentApi = {
  listDeposits: async (requesterId?: string): Promise<ListDepositsResponse> => {
    const response = await httpClient.get<ListDepositsResponse>("/payments/deposits", {
      params: requesterId ? { requester_id: requesterId } : undefined,
    });
    return response.data;
  },
  approveDeposit: async (payload: ApproveDepositRequest): Promise<void> => {
    await httpClient.post("/payments/deposits/approve", payload);
  },
  rejectDeposit: async (payload: RejectDepositRequest): Promise<void> => {
    await httpClient.post("/payments/deposits/reject", payload);
  },
  requestFiatExchange: async (payload: FiatExchangeRequest): Promise<FiatExchangeResponse> => {
    const response = await httpClient.post<FiatExchangeResponse>("/payments/deposits/fiat-exchange", payload);
    return response.data;
  },
  listEscrows: async (requesterId?: string): Promise<ListEscrowsResponse> => {
    const response = await httpClient.get<ListEscrowsResponse>("/payments/escrows", {
      params: requesterId ? { requester_id: requesterId } : undefined,
    });
    return response.data;
  },
  approveEscrow: async (payload: ApproveEscrowRequest): Promise<ApproveEscrowResponse> => {
    const response = await httpClient.post<ApproveEscrowResponse>("/payments/escrows/approve", payload);
    return response.data;
  },
  rejectEscrow: async (payload: RejectEscrowRequest): Promise<void> => {
    await httpClient.post("/payments/escrows/reject", payload);
  },
  listRedeems: async (requesterId?: string): Promise<ListRedeemsResponse> => {
    const response = await httpClient.get<ListRedeemsResponse>("/payments/redeems", {
      params: requesterId ? { requester_id: requesterId } : undefined,
    });
    return response.data;
  },
  approveRedeem: async (payload: ApproveRedeemRequest): Promise<ApproveRedeemResponse> => {
    const response = await httpClient.post<ApproveRedeemResponse>("/payments/redeems/approve", payload);
    return response.data;
  },
  rejectRedeem: async (payload: RejectRedeemRequest): Promise<void> => {
    await httpClient.post("/payments/redeems/reject", payload);
  },
};
