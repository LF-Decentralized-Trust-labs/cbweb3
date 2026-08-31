// SPDX-License-Identifier: Apache-2.0

export interface DecryptTransactionRequest {
  txHash: string;
  viewKey: string;
  reason: string;
}

export interface DecryptTransactionResponse {
  txHash: string;
  amount: string;
  currency: string;
  sender: string;
  receiver: string;
  decryptedAt: string;
}

// AuditLogEntry mirrors the backend AuditRecord JSON shape from GET /api/v1/compliance/audit/logs.
export interface AuditLogEntry {
  log_id: string;
  timestamp: string;
  actor: string;
  actor_address?: string;
  actor_name?: string;
  action: string;
  target_subject?: string;
  category: string;
  severity: string;
  outcome: string;
  details?: string;
}

export interface AuditLogsResponse {
  logs: AuditLogEntry[];
  page: number;
  limit: number;
}
