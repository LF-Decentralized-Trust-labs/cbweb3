// SPDX-License-Identifier: Apache-2.0

import type {
  DisclosureRequest,
  OpenDisclosureRequest,
  SignDisclosureRequest,
} from "../../types";
import { apiFetch } from "./apiClient";

interface DisclosureRequestResponse {
  request_id: string;
  requested_by_bank_id: string;
  target_transaction_ref: string;
  reason_code: string;
  state: string;
  quorum_required: number;
  quorum_reached: number;
  opened_at: string;
  expires_at: string;
  closed_at: string | null;
}

function toDisclosureRequest(r: DisclosureRequestResponse): DisclosureRequest {
  return {
    requestId: r.request_id,
    requestedByBankId: r.requested_by_bank_id,
    targetTransactionRef: r.target_transaction_ref,
    reasonCode: r.reason_code,
    state: r.state as DisclosureRequest["state"],
    quorumRequired: r.quorum_required,
    quorumReached: r.quorum_reached,
    openedAt: r.opened_at,
    expiresAt: r.expires_at,
    closedAt: r.closed_at,
  };
}

export const oversightApi = {
  openDisclosure: (payload: OpenDisclosureRequest): Promise<DisclosureRequest> =>
    apiFetch<DisclosureRequestResponse>("/api/v2/oversight/disclosure-request", {
      method: "POST",
      body: JSON.stringify({
        tx_ref: payload.txRef,
        requestor_id: payload.requestorId,
        reason_code: payload.reasonCode,
      }),
    }).then(toDisclosureRequest),

  signDisclosure: (payload: SignDisclosureRequest): Promise<void> =>
    apiFetch<void>("/api/v2/oversight/disclosure-sign", {
      method: "POST",
      body: JSON.stringify({
        request_id: payload.requestId,
        signer_id: payload.signerId,
      }),
    }),

  getDisclosureStatus: (requestId: string): Promise<DisclosureRequest> =>
    apiFetch<DisclosureRequestResponse>(
      `/api/v2/oversight/disclosure-status/${encodeURIComponent(requestId)}`,
    ).then(toDisclosureRequest),
};
