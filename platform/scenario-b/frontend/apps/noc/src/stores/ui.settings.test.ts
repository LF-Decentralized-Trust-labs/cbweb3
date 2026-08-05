// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import type { NocAlert } from "../types";
import { MIN_POLLING_SECONDS, filterActiveAlerts, parsePollingInterval } from "./ui.settings";

function alert(overrides: Partial<NocAlert>): NocAlert {
  return {
    id: "a1",
    component_id: "c1",
    severity: "INFO",
    state: "ACTIVE",
    title: "test alert",
    root_cause_sig: "sig",
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("parsePollingInterval", () => {
  it("accepts the documented minimum", () => {
    expect(parsePollingInterval(String(MIN_POLLING_SECONDS))).toBe(MIN_POLLING_SECONDS);
  });

  it("accepts the documented default", () => {
    expect(parsePollingInterval("15")).toBe(15);
  });

  it("rejects values below the minimum", () => {
    expect(parsePollingInterval("4")).toBeNull();
    expect(parsePollingInterval("0")).toBeNull();
    expect(parsePollingInterval("-10")).toBeNull();
  });

  it("rejects non-numeric input", () => {
    expect(parsePollingInterval("")).toBeNull();
    expect(parsePollingInterval("abc")).toBeNull();
  });
});

describe("filterActiveAlerts", () => {
  const alerts: NocAlert[] = [
    alert({ id: "info", severity: "INFO" }),
    alert({ id: "warning", severity: "WARNING" }),
    alert({ id: "high", severity: "HIGH" }),
    alert({ id: "critical", severity: "CRITICAL" }),
    alert({ id: "resolved-critical", severity: "CRITICAL", state: "RESOLVED" }),
  ];

  it("drops resolved alerts regardless of the setting", () => {
    expect(filterActiveAlerts(alerts, false).map((a) => a.id)).not.toContain("resolved-critical");
    expect(filterActiveAlerts(alerts, true).map((a) => a.id)).not.toContain("resolved-critical");
  });

  it("keeps every active severity when the setting is off", () => {
    expect(filterActiveAlerts(alerts, false).map((a) => a.id)).toEqual(["info", "warning", "high", "critical"]);
  });

  it("keeps only HIGH and CRITICAL when the setting is on", () => {
    expect(filterActiveAlerts(alerts, true).map((a) => a.id)).toEqual(["high", "critical"]);
  });
});
