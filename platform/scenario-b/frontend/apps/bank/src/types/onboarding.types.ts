// SPDX-License-Identifier: Apache-2.0

export type OnboardingRole = "ROLE_COMMERCIAL_BANK" | "ROLE_TREASURY_BANK";

export type OnboardingRequestStatus =
  | "NONE"
  | "PENDING"
  | "APPROVED"
  | "ACTIVE"
  | "FROZEN"
  | "REVOKED"
  | "REJECTED"
  | "CREDENTIAL_REQUESTED"
  | "KYC_APPROVED";

export type InitiateOnboardingPayload = {
  institution_name: string;
  country: string;
  role: OnboardingRole;
  email: string;
  username: string;
};

export type InitiateOnboardingResponse = {
  request_id: string;
  user_id: string;
  wallet_address: string;
  status: OnboardingRequestStatus;
};

export type OnboardingStatusResponse = {
  status: OnboardingRequestStatus;
  pop_nonce?: string;
};

export type OnboardingMyStatusResponse = {
  request_id: string;
  user_id: string;
  status: OnboardingRequestStatus;
};

export type CompleteOnboardingPayload = {
  request_id: string;
  user_id: string;
};

export type CompleteOnboardingResponse = {
  user_id: string;
  wallet_address: string;
  tx_hash: string;
  client_secret: string;
  status: OnboardingRequestStatus;
  access_token?: string;
  pki_login_error?: string | null;
};