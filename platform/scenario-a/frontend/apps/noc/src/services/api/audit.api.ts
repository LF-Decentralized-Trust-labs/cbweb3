// SPDX-License-Identifier: Apache-2.0

import type { AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

type DataEnvelope<T> = { data: T };

export const auditApi = {
  list: async (limit = 100, offset = 0): Promise<AuditLogEntry[]> => {
    const res = await httpClient.get<DataEnvelope<AuditLogEntry[]>>(
      `/audit?limit=${limit}&offset=${offset}`,
    );
    return res.data.data;
  },
};
