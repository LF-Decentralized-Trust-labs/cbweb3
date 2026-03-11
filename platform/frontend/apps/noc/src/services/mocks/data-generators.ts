import type { TelemetryFrame } from "../../types";

const random = (min: number, max: number) => Math.floor(Math.random() * (max - min + 1)) + min;

export const generateTelemetryFrame = (component: TelemetryFrame["component"], componentId: string): TelemetryFrame => ({
  id: `tm_${Math.random().toString(36).slice(2, 10)}`,
  component,
  componentId,
  cpuPct: random(30, 96),
  memoryPct: random(30, 95),
  latencyMs: random(20, 420),
  healthy: Math.random() > 0.08,
  timestamp: new Date().toISOString(),
});
