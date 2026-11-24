// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { formatLogLine } from "./structured-log";

test("formatLogLine writes one JSON object with timestamp, severity, service and message", () => {
  const line = formatLogLine("warn", "stop failed", { spoke: "spoke-a" }, new Date("2026-01-02T03:04:05.000Z"));
  assert.ok(!line.includes("\n"));
  assert.deepEqual(JSON.parse(line), {
    ts: "2026-01-02T03:04:05.000Z",
    severity: "warn",
    service: "cacti-liquidity-relay",
    msg: "stop failed",
    spoke: "spoke-a",
  });
});

test("formatLogLine does not let an extra field overwrite the fixed ones", () => {
  const parsed = JSON.parse(formatLogLine("error", "real", { severity: "info", msg: "fake" }));
  assert.equal(parsed.severity, "error");
  assert.equal(parsed.msg, "real");
});
