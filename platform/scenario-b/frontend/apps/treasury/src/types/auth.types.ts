// SPDX-License-Identifier: Apache-2.0

export type Role = "TREASURY";

export type TreasuryUser = {
  /**
   * The realm roles the session's token carries, from /auth/me. `role` below is a fixed
   * label, not a claim: authorization must read this.
   */
  roles: string[];
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
