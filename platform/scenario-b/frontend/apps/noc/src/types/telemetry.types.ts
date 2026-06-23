// SPDX-License-Identifier: Apache-2.0

export type ComponentKind = "BESU" | "PALADIN" | "CACTI";

export type TelemetryFrame = {
  id: string;
  component: ComponentKind;
  componentId: string;
  cpuPct: number;
  memoryPct: number;
  latencyMs: number;
  healthy: boolean;
  timestamp: string;
};
