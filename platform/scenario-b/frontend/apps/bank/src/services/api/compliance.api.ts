// SPDX-License-Identifier: Apache-2.0

import { mockDb } from "../mocks/mock-db";

export const complianceApi = {
  getCredentials: () => mockDb.getCredentials(),
  attachCredentials: (transactionId: string, credentialIds: string[]) =>
    mockDb.attachCredentials(transactionId, credentialIds),
};
