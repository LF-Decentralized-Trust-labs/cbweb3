export type NocContainerLog = {
  id: number;
  component_id: string;
  stream: "stdout" | "stderr";
  log_line: string;
  occurred_at: string;
  received_at: string;
};
