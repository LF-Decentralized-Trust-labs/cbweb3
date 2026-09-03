// SPDX-License-Identifier: Apache-2.0

import type {
  BalanceResponse,
  CreateAmountRequest,
  ListDepositsResponse,
  ListEscrowsResponse,
  ListRedeemsResponse,
  RegisterDepositResponse,
  RequestEscrowResponse,
  RequestRedeemResponse,
} from "../../types";
import { httpClient } from "./http-client";

export const paymentApi = {
  registerDeposit: async (payload: CreateAmountRequest): Promise<RegisterDepositResponse> => {
    const response = await httpClient.post<RegisterDepositResponse>("/payments/deposits", payload);
    return response.data;
  },
  listDeposits: async (): Promise<ListDepositsResponse> => {
    const response = await httpClient.get<ListDepositsResponse>("/payments/deposits");
    return response.data;
  },
  requestEscrow: async (payload: CreateAmountRequest): Promise<RequestEscrowResponse> => {
    const response = await httpClient.post<RequestEscrowResponse>("/payments/escrows", payload);
    return response.data;
  },
  listEscrows: async (): Promise<ListEscrowsResponse> => {
    const response = await httpClient.get<ListEscrowsResponse>("/payments/escrows");
    return response.data;
  },
  requestRedeem: async (payload: CreateAmountRequest): Promise<RequestRedeemResponse> => {
    const response = await httpClient.post<RequestRedeemResponse>("/payments/redeems", payload);
    return response.data;
  },
  listRedeems: async (): Promise<ListRedeemsResponse> => {
    const response = await httpClient.get<ListRedeemsResponse>("/payments/redeems");
    return response.data;
  },
  getBalance: async (): Promise<BalanceResponse> => {
    const response = await httpClient.get<BalanceResponse>("/token/balance");
    return response.data;
  },
  getFiatBalance: async (): Promise<BalanceResponse> => {
    const response = await httpClient.get<BalanceResponse>("/token/fiat-balance");
    return response.data;
  },
};
