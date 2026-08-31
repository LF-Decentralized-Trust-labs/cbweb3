// SPDX-License-Identifier: Apache-2.0

import type { Movement, StatementResponse } from "../../types";
import { httpClient } from "./http-client";

export const statementApi = {
  list: async (): Promise<Movement[]> => {
    const response = await httpClient.get<StatementResponse>("/statement");
    return response.data.movements || [];
  },
};
