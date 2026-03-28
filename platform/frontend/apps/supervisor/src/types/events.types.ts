export type SupervisorEventType = "POOL_IMBALANCE" | "AUDIT_ACTIVITY" | "REGISTRY_STATUS";

export interface SupervisorEvent {
  id: string;
  type: SupervisorEventType;
  message: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  createdAt: string;
}
