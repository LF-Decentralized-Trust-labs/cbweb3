// SPDX-License-Identifier: Apache-2.0

import type { NocAlert } from "../types";

/** Lowest polling interval an operator may configure, in seconds. */
export const MIN_POLLING_SECONDS = 5;

/**
 * Parses an operator-entered polling interval. Returns null when the value is
 * not a whole number of seconds or is below MIN_POLLING_SECONDS, so callers can
 * reject the save instead of silently keeping the previous interval.
 */
export function parsePollingInterval(raw: string): number | null {
  const parsed = Number.parseInt(raw, 10);
  if (Number.isNaN(parsed) || parsed < MIN_POLLING_SECONDS) {
    return null;
  }
  return parsed;
}

/**
 * Active alerts for the Dashboard feed. When criticalOnly is set (the "Critical
 * Alerts Only" setting), INFO and WARNING alerts are hidden.
 */
export function filterActiveAlerts(alerts: NocAlert[], criticalOnly: boolean): NocAlert[] {
  return alerts.filter(
    (alert) =>
      alert.state === "ACTIVE" && (!criticalOnly || alert.severity === "HIGH" || alert.severity === "CRITICAL"),
  );
}
