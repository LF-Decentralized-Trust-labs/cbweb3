// SPDX-License-Identifier: Apache-2.0

import type { DepositRecord, FundingDecisionPayload, FundingRequest, FundingRequestStatus } from "../../types";
import { normalizePaymentStatus, PaymentStatus } from "../../types";
import { paymentApi } from "./payment.api";

const mapStatus = (status: DepositRecord["status"]): FundingRequestStatus => {
  const normalized = normalizePaymentStatus(status);
  if (normalized === PaymentStatus.APPROVED) return "APPROVED";
  if (normalized === PaymentStatus.REJECTED) return "REJECTED";
  if (normalized === PaymentStatus.MINT_FAILED) return "REJECTED";
  return "PENDING";
};

const mapDeposit = (deposit: DepositRecord): FundingRequest => ({
  id: deposit.id,
  institutionId: deposit.requester_id,
  institutionName: deposit.requester_id,
  amount: deposit.amount,
  fiatProofRef: deposit.requester_paladin_identity,
  justification: deposit.rejection_reason || "",
  status: mapStatus(deposit.status),
  submittedAt: deposit.created_at,
  reviewReason: deposit.rejection_reason || undefined,
});

export const fundingApi = {
  list: async (): Promise<FundingRequest[]> => {
    const response = await paymentApi.listDeposits();
    return response.deposits.map(mapDeposit);
  },
  approve: async (payload: FundingDecisionPayload): Promise<FundingRequest> => {
    await paymentApi.approveDeposit({ deposit_id: payload.requestId });
    const response = await paymentApi.listDeposits();
    const deposit = response.deposits.find((entry) => entry.id === payload.requestId);
    if (!deposit) {
      throw new Error("Deposit not found after approval");
    }
    return mapDeposit(deposit);
  },
  reject: async (payload: FundingDecisionPayload): Promise<FundingRequest> => {
    await paymentApi.rejectDeposit({ deposit_id: payload.requestId, reason: payload.reason ?? "" });
    const response = await paymentApi.listDeposits();
    const deposit = response.deposits.find((entry) => entry.id === payload.requestId);
    if (!deposit) {
      throw new Error("Deposit not found after rejection");
    }
    return mapDeposit(deposit);
  },
};
