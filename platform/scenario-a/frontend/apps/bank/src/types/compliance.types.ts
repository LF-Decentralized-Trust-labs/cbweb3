// SPDX-License-Identifier: Apache-2.0

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
