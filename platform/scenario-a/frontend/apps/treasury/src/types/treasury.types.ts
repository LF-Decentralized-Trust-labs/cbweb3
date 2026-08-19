// SPDX-License-Identifier: Apache-2.0

export type TreasuryOperationKind = "MINT" | "BURN";

export type TreasuryOperationStatus = "PENDING" | "CONFIRMED" | "FAILED";

export type MintPayload = {
  requestId: string;
  targetInstitutionId: string;
  amount: string;
  reserveProofRef: string;
};

export type BurnPayload = {
  sourceAccount: string;
  amount: string;
  reason: string;
};

export type TreasuryOperation = {
  id: string;
  kind: TreasuryOperationKind;
  amount: string;
  status: TreasuryOperationStatus;
  createdAt: string;
  reference?: string;
};

export type SupplySnapshot = {
  circulatingSupply: string;
  updatedAt: string;
};

