export type ComponentHealthStatus = "HEALTHY" | "DEGRADED" | "OFFLINE" | "UNKNOWN";
export type ComponentType = "BESU" | "CACTI_RELAY" | "PALADIN" | "PAYMENT_ORCHESTRATOR";

export type NocComponent = {
  id: string;
  agent_id: string;
  spoke_id: string;
  name: string;
  type: ComponentType;
  endpoint: string;
  health_status: ComponentHealthStatus;
  last_checked_at: string | null;
  last_block_number: number | null;
  created_at: string;
};

export type NocHealthEvent = {
  id: string;
  component_id: string;
  status: ComponentHealthStatus;
  occurred_at: string;
  received_at: string;
  diagnostic: string | null;
  block_number: number | null;
};
