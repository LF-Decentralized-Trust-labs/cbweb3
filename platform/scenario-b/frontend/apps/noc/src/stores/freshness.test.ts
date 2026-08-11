// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { STALE_MIN_SECONDS, isDataStale, snapshotAgeSeconds } from "./freshness.store";

describe("isDataStale", () => {
  const now = 1_800_000_000_000;

  it("is not stale in the moments before the first reply arrives", () => {
    expect(isDataStale(null, now, 15)).toBe(false);
  });

  it("is stale when the first call already failed, so it never claims to be live", () => {
    expect(isDataStale(null, now, 15, now - 1000)).toBe(true);
  });

  it("is not stale within three polling cycles", () => {
    expect(isDataStale(now - 40_000, now, 15)).toBe(false);
  });

  it("is stale past three polling cycles", () => {
    expect(isDataStale(now - 46_000, now, 15)).toBe(true);
  });

  it("never flips before the minimum window, even at the 5s interval", () => {
    expect(isDataStale(now - (STALE_MIN_SECONDS - 1) * 1000, now, 5)).toBe(false);
    expect(isDataStale(now - (STALE_MIN_SECONDS + 1) * 1000, now, 5)).toBe(true);
  });
});

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
