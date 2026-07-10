// SPDX-License-Identifier: Apache-2.0

import type { ListIdentitiesResponse } from "../../types";
import { httpClient } from "./http-client";

export const identityApi = {
  list: async (): Promise<string[]> => {
    const response = await httpClient.get<ListIdentitiesResponse>("/identities");
    return response.data.identities || [];
  },
};
