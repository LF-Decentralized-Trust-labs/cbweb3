// SPDX-License-Identifier: Apache-2.0

export interface TokenBalance {
  publicBalance: string;
  privateBalance: string;
  currency: string;
  updatedAt: string;
}

export type LiquidityRequestType = "ON_RAMP" | "OFF_RAMP";

export interface OnRampRequestPayload {
  type: LiquidityRequestType;
  amount: string;
  fiatProofRef: string;
  justification: string;
}

export type OnRampRequestStatus = "PENDING" | "APPROVED" | "REJECTED" | "COMPLETED";

export interface OnRampRequest {
  id: string;
  type: LiquidityRequestType;
  amount: string;
  fiatProofRef: string;
  justification: string;
  status: OnRampRequestStatus;
  createdAt: string;
}

export interface TransferRequest {
  toAddress: string;
  amount: string;
  shielded: boolean;
}

export interface TokenTransaction {
  id: string;
  kind: "TRANSFER" | "ON_RAMP_REQUEST";
  amount: string;
  status: "PENDING" | "CONFIRMED" | "FAILED";
  createdAt: string;
}
