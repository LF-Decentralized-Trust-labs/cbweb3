// SPDX-License-Identifier: Apache-2.0

export type AccountEntry = {
  id: string;
  participantId: string;
  participantName: string;
  frozen: boolean;
  frozenAt: string | null;
  frozenReason: string | null;
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
