// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from "vitest";
import { formatLogLine } from "./structured-log";

describe("formatLogLine", () => {
  it("writes one JSON object with timestamp, severity, service and message", () => {
    const line = formatLogLine("warn", "closing failed", { spoke: "spoke-a" }, new Date("2026-01-02T03:04:05.000Z"));
    expect(line).not.toContain("\n");
    expect(JSON.parse(line)).toEqual({
      ts: "2026-01-02T03:04:05.000Z",
      severity: "warn",
      service: "cacti-htlc-relay",
      msg: "closing failed",
      spoke: "spoke-a",
    });
  });

  it("does not let an extra field overwrite the fixed ones", () => {
    const parsed = JSON.parse(formatLogLine("error", "real", { severity: "info", msg: "fake" }));
    expect(parsed.severity).toBe("error");
    expect(parsed.msg).toBe("real");
  });
});
