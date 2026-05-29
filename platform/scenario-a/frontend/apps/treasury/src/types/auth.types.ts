export type Role = "TREASURY";

export type TreasuryUser = {
  id: string;
  name: string;
  institutionId: string;
  role: Role;
  walletAddress: string;
  authorizedIssuer: boolean;
};

export type LoginRequest = {
  username: string;
  password: string;
};

export type LoginResponse = {
  user: TreasuryUser;
};
