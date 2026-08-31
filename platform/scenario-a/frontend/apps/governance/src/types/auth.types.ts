// SPDX-License-Identifier: Apache-2.0

export type LoginRequest = {
  username: string;
  password: string;
};

export type TokenResponse = {
  accessToken: string;
  refreshToken: string;
  tokenType: string;
  expiresIn: number;
};

export type PkiChallengeResponse = {
  nonce: string;
};

export type LoginResponse = TokenResponse | PkiChallengeResponse;

export type UserProfile = {
  subject: string;
  issuer: string;
  roles: string[];
  wallet?: string;
  country?: string;
  bankId?: string;
  privacyGroup?: string;
};
