// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { formatAmountString, formatTokenAmount } from "./format";

describe("formatTokenAmount", () => {
  it("is exact above the double precision limit", () => {
    // 2^53 is where a double stops being able to count by one. The old
    // Number(bigint) path rounded here; the operator saw a figure that was not
    // the balance.
    const beyondDouble = 9_007_199_254_740_993n; // 2^53 + 1
    expect(formatTokenAmount(beyondDouble)).toBe("9,007,199,254,740,993");
    expect(formatTokenAmount(beyondDouble)).not.toBe(Number(beyondDouble).toLocaleString("en-US"));
  });

  it("is exact for an 18-decimal token balance", () => {
    // A realistic wei-scale figure: one million tokens at 18 decimals.
    expect(formatTokenAmount(1_000_000_000_000_000_000_000_000n)).toBe("1,000,000,000,000,000,000,000,000");
  });

  it("formats ordinary values with thousands separators", () => {
    expect(formatTokenAmount(0n)).toBe("0");
    expect(formatTokenAmount(50_000n)).toBe("50,000");
  });
});

describe("formatAmountString", () => {
  it("is exact for a value the double path would round", () => {
    // Number("90071992547409931") rounds; BigInt does not.
    expect(formatAmountString("90071992547409931")).toBe("90,071,992,547,409,931");
    expect(formatAmountString("90071992547409931")).not.toBe(Number("90071992547409931").toLocaleString("en-US"));
  });

  it("formats ordinary integers", () => {
    expect(formatAmountString("50000")).toBe("50,000");
    expect(formatAmountString("0")).toBe("0");
    expect(formatAmountString("-25")).toBe("-25");
  });

  it("returns the raw string for anything that is not a plain integer", () => {
    // Showing the value as received beats crashing the screen, and beats
    // rendering a rounded number that looks authoritative.
    expect(formatAmountString("")).toBe("");
    expect(formatAmountString("12.5")).toBe("12.5");
    expect(formatAmountString("n/a")).toBe("n/a");
  });
});
