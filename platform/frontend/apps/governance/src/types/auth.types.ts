export type GovernanceRole = "CENTRAL_BANK_ADMIN";

export type GovernanceUser = {
  id: string;
  username: string;
  displayName: string;
  role: GovernanceRole;
};

export type AuthClaims = {
  sub: string;
  role: GovernanceRole;
};

export type LoginRequest = {
  username: string;
  password: string;
};

export type LoginResponse = {
  token: string;
  user: GovernanceUser;
};
