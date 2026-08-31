// SPDX-License-Identifier: Apache-2.0

export type SupervisorEventType = "HTLC_TIMEOUT" | "AUDIT_ACTIVITY" | "REGISTRY_STATUS";

export interface SupervisorEvent {
  id: string;
  type: SupervisorEventType;
  message: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  createdAt: string;
}
