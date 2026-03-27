export type OnboardingRole = "ROLE_COMMERCIAL_BANK" | "ROLE_TREASURY_BANK";

export type OnboardingRequestStatus = "CREDENTIAL_REQUESTED" | "KYC_APPROVED" | "ACTIVE" | "REVOKED";

export type InitiateOnboardingPayload = {
  institution_name: string;
  bank_code: string;
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