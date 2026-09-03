// SPDX-License-Identifier: Apache-2.0

export type AuditLogEntry = {
  id: string;
  actor: string;
  action: string;
  target_id: string;
  target_type: string;
  detail: string;
  created_at: string;
};

export type AuditFilter = {
  actor?: string;
  action?: string;
};
