export type ParticipantStatus = "ACTIVE" | "PENDING" | "CREDENTIAL_REQUESTED" | "KYC_APPROVED" | "REVOKED" | "FROZEN";

export type Participant = {
  id: string;
  name: string;
  cnpj: string;
  status: ParticipantStatus;
  credentialId: string | null;
  credentialExpiry: string | null;
  createdAt: string;
};

export type IssueCredentialPayload = {
  entityName: string;
  cnpj: string;
  scopes: string[];
  reason: string;
};

export type CredentialIssuanceResult = {
  participantId: string;
  credentialId: string;
  credentialExpiry: string;
};

export type KycRequestStatus = "CREDENTIAL_REQUESTED" | "KYC_APPROVED" | "ACTIVE" | "REVOKED";

export type KycStatusEntry = {
  subject: string;
  status: KycRequestStatus;
  institution_name?: string;
  bank_code?: string;
  country?: string;
  wallet_address?: string;
  created_at?: string;
};

export type PendingKycParticipantApi = {
  user_id: string;
  institution_name?: string;
  cnpj?: string;
  bank_code?: string;
  country_code?: string;
  role?: string;
  wallet_address?: string;
  status: KycRequestStatus;
};

export type PendingKycApiResponse = {
  participants: PendingKycParticipantApi[];
};

export type ApproveKycPayload = {
  subject: string;
  reason: string;
};

export type ApproveKycResponse = {
  status: string;
  pop_nonce: string;
};
