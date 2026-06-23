// SPDX-License-Identifier: Apache-2.0

export type AlertSeverity = "INFO" | "WARNING" | "HIGH" | "CRITICAL";
export type AlertState = "ACTIVE" | "RESOLVED";

export type NocAlert = {
  id: string;
  component_id: string;
  incident_id?: string;
  severity: AlertSeverity;
  state: AlertState;
  title: string;
  root_cause_sig: string;
  created_at: string;
  resolved_at?: string;
  acknowledged_by?: string;
};

export type NocAlertDetail = NocAlert & {
  component: {
    id: string;
    name: string;
    type: string;
    endpoint: string;
    health_status: string;
    spoke_id: string;
    last_checked_at?: string;
    last_block_number?: number;
  };
};

/** Legacy alert shape retained for mock-backed stores */
export type LegacyNocAlert = {
  id: string;
  source: "INFRASTRUCTURE" | "RELAY" | "POOL" | "TELEMETRY";
  message: string;
  severity: AlertSeverity;
  correlationId?: string;
  createdAt: string;
};
