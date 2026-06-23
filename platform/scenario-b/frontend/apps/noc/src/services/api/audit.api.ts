// SPDX-License-Identifier: Apache-2.0

import type { AuditLogEntry } from "../../types";
import { httpClient } from "./http-client";

type DataEnvelope<T> = { data: T };

export const auditApi = {
  list: async (): Promise<AuditLogEntry[]> => {
    const res = await httpClient.get<DataEnvelope<AuditLogEntry[]>>("/audit");
    return res.data.data ?? [];
  },
};
