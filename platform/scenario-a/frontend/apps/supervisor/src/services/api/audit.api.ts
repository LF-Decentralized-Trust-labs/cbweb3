// SPDX-License-Identifier: Apache-2.0

import type { AuditLogsResponse, DecryptTransactionRequest, DecryptTransactionResponse } from "../../types";
import { apiFetch } from "./apiClient";

export interface AuditLogsParams {
  category?: string;
  severity?: string;
  from_date?: string;
  to_date?: string;
  page?: number;
  limit?: number;
}

// Raw snake_case shape returned by the backend decrypt endpoint.
interface DecryptTransactionRaw {
  tx_hash: string;
  amount: string;
  currency: string;
  sender: string;
  receiver: string;
  decrypted_at: string;
}

export const auditApi = {
  decryptTransaction: async (payload: DecryptTransactionRequest): Promise<DecryptTransactionResponse> => {
    const raw = await apiFetch<DecryptTransactionRaw>("/api/v1/compliance/decrypt-transaction", {
      method: "POST",
      // Backend expects snake_case field names.
      body: JSON.stringify({
        tx_hash: payload.txHash,
        view_key: payload.viewKey,
        reason: payload.reason,
      }),
    });
    return {
      txHash: raw.tx_hash,
      amount: raw.amount,
      currency: raw.currency,
      sender: raw.sender,
      receiver: raw.receiver,
      decryptedAt: raw.decrypted_at,
    };
  },

  getAuditLogs: (params: AuditLogsParams = {}): Promise<AuditLogsResponse> => {
    const qs = new URLSearchParams();
    if (params.category) qs.set("category", params.category);
    if (params.severity) qs.set("severity", params.severity);
    if (params.from_date) qs.set("from_date", params.from_date);
    if (params.to_date) qs.set("to_date", params.to_date);
    if (params.page != null) qs.set("page", String(params.page));
    if (params.limit != null) qs.set("limit", String(params.limit));
    const query = qs.toString();
    return apiFetch<AuditLogsResponse>(`/api/v1/compliance/audit/logs${query ? `?${query}` : ""}`);
  },
};
