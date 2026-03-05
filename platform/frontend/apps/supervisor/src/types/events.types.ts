export type SupervisorEventType = "POOL_IMBALANCE" | "CIRCUIT_BREAKER" | "PARTICIPANT_ONBOARDED" | "CREDENTIAL_REVOKED";

export interface SupervisorEvent {
  id: string;
  type: SupervisorEventType;
  message: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  createdAt: string;
}
