export type IssueCredentialPayload = {
  institutionId: string;
  institutionName: string;
  credentialType: "AUTHORIZED_ISSUER" | "KYC_VERIFIED";
};

export type IssuedCredential = {
  id: string;
  institutionId: string;
  institutionName: string;
  credentialType: IssueCredentialPayload["credentialType"];
  issuedAt: string;
  issuedBy: string;
};
