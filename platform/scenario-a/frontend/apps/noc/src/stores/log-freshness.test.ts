// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { snapshotAgeSeconds } from "./log-freshness";

describe("snapshotAgeSeconds", () => {
  const now = Date.parse("2026-08-04T16:00:00.000Z");

  it("measures the age of the newest collected line", () => {
    expect(snapshotAgeSeconds("2026-08-04T15:59:15.000Z", now)).toBe(45);
  });

  it("never reports a negative age when the agent clock runs ahead", () => {
    expect(snapshotAgeSeconds("2026-08-04T16:00:30.000Z", now)).toBe(0);
  });

  it("returns null when there is no timestamp to date", () => {
    expect(snapshotAgeSeconds(undefined, now)).toBeNull();
    expect(snapshotAgeSeconds("not-a-date", now)).toBeNull();
  });
});
