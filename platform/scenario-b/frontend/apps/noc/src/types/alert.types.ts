export type AlertSeverity = "INFO" | "WARNING" | "CRITICAL";

export type NocAlert = {
  id: string;
  source: "INFRASTRUCTURE" | "RELAY" | "POOL" | "TELEMETRY";
  message: string;
  severity: AlertSeverity;
  correlationId?: string;
  createdAt: string;
};
