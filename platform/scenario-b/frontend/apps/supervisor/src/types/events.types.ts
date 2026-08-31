// SPDX-License-Identifier: Apache-2.0

export type SupervisorEventType = "POOL_IMBALANCE" | "AUDIT_ACTIVITY" | "REGISTRY_STATUS" | "CIRCUIT_BREAKER";

export interface SupervisorEvent {
  id: string;
  type: SupervisorEventType;
  message: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  createdAt: string;
}
