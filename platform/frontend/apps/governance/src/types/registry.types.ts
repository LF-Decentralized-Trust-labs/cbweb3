export type ParticipantStatus = "ACTIVE" | "PENDING" | "REVOKED" | "FROZEN";

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
