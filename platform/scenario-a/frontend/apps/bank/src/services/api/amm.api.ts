// SPDX-License-Identifier: Apache-2.0

import type { AMMQuoteRequest } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const ammApi = {
  quoteExactOutput: (payload: AMMQuoteRequest) => mockDb.quoteExactOutput(payload),
  getPoolStatus: () => mockDb.getPoolStatus(),
  swapExactOutput: (payload: AMMQuoteRequest) => mockDb.swapExactOutput(payload),
};
