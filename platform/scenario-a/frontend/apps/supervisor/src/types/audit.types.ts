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

export interface AuditLogEntry {
  id: string;
  actor: string;
  action: string;
  target: string;
  timestamp: string;
  status: "SUCCESS" | "FAILED";
}
