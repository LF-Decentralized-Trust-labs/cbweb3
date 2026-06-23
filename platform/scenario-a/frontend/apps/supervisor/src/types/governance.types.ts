// SPDX-License-Identifier: Apache-2.0

export const InstitutionType = {
  COMMERCIAL_BANK: "COMMERCIAL_BANK",
  CENTRAL_BANK: "CENTRAL_BANK",
  CLEARING_HOUSE: "CLEARING_HOUSE",
} as const;

export type InstitutionType = (typeof InstitutionType)[keyof typeof InstitutionType];

export const CredentialStatus = {
  ACTIVE: "ACTIVE",
  REVOKED: "REVOKED",
  PENDING: "PENDING",
} as const;

export type CredentialStatus = (typeof CredentialStatus)[keyof typeof CredentialStatus];

export interface InstitutionalOnboardingRequest {
  institutionName: string;
  publicKey: string;
  evmAddress: string;
  jurisdictionCode: string;
  institutionType: InstitutionType;
}

export interface CredentialRevocationRequest {
  address: string;
  reason: string;
  effectiveDate?: string;
}

export interface CredentialRevocationResponse {
  address: string;
  revokedAt: string;
  reason: string;
  effectiveDate: string;
}

export interface SanctionsListEntry {
  address: string;
  entityName: string;
  reason: string;
  addedAt: string;
  source: string;
}

export interface SanctionsCheckResponse {
  address: string;
  isSanctioned: boolean;
  matches: SanctionsListEntry[];
  checkedAt: string;
}

export interface Participant {
  id: string;
  institutionName: string;
  evmAddress: string;
  publicKey: string;
  credentialStatus: CredentialStatus;
  jurisdictionCode: string;
  institutionType: InstitutionType;
  onboardedAt: string;
  lastActivityAt: string;
}
