export type FundingRequestStatus = "PENDING" | "APPROVED" | "REJECTED" | "COMPLETED";

export type FundingRequest = {
  id: string;
  institutionId: string;
  institutionName: string;
  amount: string;
  fiatProofRef: string;
  justification: string;
  status: FundingRequestStatus;
  submittedAt: string;
  reviewedAt?: string;
  reviewedBy?: string;
  reviewReason?: string;
};

export type FundingDecisionPayload = {
  requestId: string;
  reason?: string;
};
