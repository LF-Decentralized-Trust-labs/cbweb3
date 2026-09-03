// SPDX-License-Identifier: Apache-2.0

export type AccountEntry = {
  id: string;
  participantId: string;
  participantName: string;
  frozen: boolean;
  frozenAt: string | null;
  frozenReason: string | null;
  // Participant role (e.g. ROLE_CENTRAL_BANK). Used to protect the Central Bank
  // itself — the admin — from being frozen/unfrozen. Optional (absent in mocks).
  role?: string;
};

export type FreezePayload = {
  accountId: string;
  reason: string;
};

export type FreezeResult = {
  success: boolean;
  accountId: string;
  frozenAt: string;
};
