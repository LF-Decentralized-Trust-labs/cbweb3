export type Role = "TREASURY";

export type TreasuryUser = {
  id: string;
  name: string;
  institutionId: string;
  role: Role;
  walletAddress: string;
  authorizedIssuer: boolean;
};

export type MeResponse = {
  subject: string;
  issuer: string;
  roles: string[];
  wallet?: string;
  country?: string;
  bankId?: string;
  privacyGroup?: string;
};

export type LoginRequest = {
  clientId: string;
  clientSecret: string;
};
