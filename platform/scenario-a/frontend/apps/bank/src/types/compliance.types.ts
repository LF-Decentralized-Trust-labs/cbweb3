export interface ComplianceCredential {
  id: string;
  type: string;
  issuer: string;
  expiresAt: string;
}

export interface AttachComplianceRequest {
  transactionId: string;
  credentialIds: string[];
}
