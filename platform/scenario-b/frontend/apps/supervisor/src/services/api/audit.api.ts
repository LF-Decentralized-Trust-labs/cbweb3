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

export const auditApi = {
  decryptTransaction: (payload: DecryptTransactionRequest): Promise<DecryptTransactionResponse> =>
    apiFetch<DecryptTransactionResponse>("/api/v1/compliance/decrypt-transaction", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

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
