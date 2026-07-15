// SPDX-License-Identifier: Apache-2.0

import type { IdentityRoster, ListIdentitiesResponse } from "../../types";
import { httpClient } from "./http-client";

export const identityApi = {
  list: async (): Promise<IdentityRoster> => {
    const response = await httpClient.get<ListIdentitiesResponse>("/identities");
    // The backend sends `configured: false` (with an empty list) when no roster
    // is available. Preserve that signal so the UI can explain the empty state
    // instead of rendering a silent empty dropdown.
    return {
      identities: response.data.identities || [],
      configured: response.data.configured ?? false,
    };
  },
};
