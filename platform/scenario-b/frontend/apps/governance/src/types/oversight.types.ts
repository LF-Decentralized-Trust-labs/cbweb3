// SPDX-License-Identifier: Apache-2.0

export const DISCLOSURE_STATE = {
  PENDING: "PENDING",
  QUORUM_REACHED: "QUORUM_REACHED",
  EXPIRED: "EXPIRED",
  REJECTED: "REJECTED",
} as const;

export type DisclosureState = (typeof DISCLOSURE_STATE)[keyof typeof DISCLOSURE_STATE];

export interface DisclosureRequest {
  request_id: string;
  tx_ref: string;
  requestor_id: string;
  reason_code: string;
  state: DisclosureState;
  quorum_reached: number;
  quorum_required: number;
  expires_at: string;
  closed_at: string | null;
}

export interface OpenDisclosureRequest {
  tx_ref: string;
  requestor_id: string;
  reason_code: string;
}

export interface SignDisclosureRequest {
  request_id: string;
  signer_id: string;
}
