export type EventType = "funding.request.updated" | "supply.updated" | "reconciliation.updated" | "audit.log.created";

export type TreasuryEvent = {
  id: string;
  type: EventType;
  message: string;
  severity: "INFO" | "WARNING" | "CRITICAL";
  createdAt: string;
};
