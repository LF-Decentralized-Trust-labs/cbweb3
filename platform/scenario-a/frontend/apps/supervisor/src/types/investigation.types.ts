export type DisclosureState = "PENDING" | "QUORUM_REACHED" | "EXPIRED";

export interface DisclosureRequest {
  requestId: string;
  requestedByBankId: string;
  targetTransactionRef: string;
  reasonCode: string;
  state: DisclosureState;
  quorumRequired: number;
  quorumReached: number;
  openedAt: string;
  expiresAt: string;
  closedAt: string | null;
}

export type DisclosureReasonCode =
  | "AML_ALERT"
  | "CFT_INVESTIGATION"
  | "COURT_ORDER"
  | "REGULATORY_EXAM";

export interface OpenDisclosureRequest {
  txRef: string;
  requestorId: string;
  reasonCode: DisclosureReasonCode;
}

export interface SignDisclosureRequest {
  requestId: string;
  signerId: string;
}
