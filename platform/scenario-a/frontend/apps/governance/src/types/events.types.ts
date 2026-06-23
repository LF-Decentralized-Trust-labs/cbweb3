// SPDX-License-Identifier: Apache-2.0

import type { AuditCategory, AuditSeverity } from "./audit.types";

export type GovernanceEvent = {
  id: string;
  category: AuditCategory;
  severity: AuditSeverity;
  message: string;
  createdAt: string;
};
