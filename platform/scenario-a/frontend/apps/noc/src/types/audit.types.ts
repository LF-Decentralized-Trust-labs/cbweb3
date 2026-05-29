export type AuditLogEntry = {
  id: string;
  actor: string;
  action: string;       // ACKNOWLEDGE_ALERT | DISMISS_ALERT
  target_id: string;
  target_type: string;
  detail: string;
  created_at: string;
};

export type AuditFilter = {
  actor?: string;
  action?: string;
};
