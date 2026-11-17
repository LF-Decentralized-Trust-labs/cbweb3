// SPDX-License-Identifier: Apache-2.0

/**
 * One structured JSON log line on stdout: ISO-8601 timestamp, severity, service and message,
 * plus any extra fields. Used where a line must be machine-readable, such as a Besu
 * WebSocket that may have been left open on exit.
 */
export type Severity = "debug" | "info" | "warn" | "error";

export const SERVICE_NAME = "cacti-liquidity-relay";

export function formatLogLine(
  severity: Severity,
  msg: string,
  fields: Record<string, unknown> = {},
  now: Date = new Date(),
): string {
  // The fixed fields come last so an extra field cannot overwrite them.
  return JSON.stringify({ ...fields, ts: now.toISOString(), severity, service: SERVICE_NAME, msg });
}

export function logJson(severity: Severity, msg: string, fields: Record<string, unknown> = {}): void {
  process.stdout.write(formatLogLine(severity, msg, fields) + "\n");
}
