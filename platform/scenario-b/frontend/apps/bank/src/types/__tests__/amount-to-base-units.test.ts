// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { parseAmount } from "@cbweb3/ui";
import { displayToBase } from "../payment.types";

// The Scenario B half: an amount that slips past the form does not merely get
// refused downstream, it settles, because displayToBase scales whatever it is
// handed.
describe("the amount that reaches displayToBase", () => {
  const DECIMALS = 18;
  const ONE = 10n ** BigInt(DECIMALS);

  // What <input type="number"> handed the form when a pt-BR operator typed
  // "1.000,10", measured in Chromium: the string "1.00010". The page's old
  // guard, /^\d+(\.\d+)?$/, accepted it — a well-formed decimal by then — and
  // displayToBase scaled it faithfully. A thousand became one.
  it("shows what the old path settled for a pt-BR operator", () => {
    const whatTheBrowserHandedUs = "1.00010";
    expect(/^\d+(\.\d+)?$/.test(whatTheBrowserHandedUs)).toBe(true);
    expect(BigInt(displayToBase(whatTheBrowserHandedUs, DECIMALS))).toBeLessThan(2n * ONE);
  });

  // And from the other side: displayToBase reads a raw comma as a thousands
  // separator, so the same figure overshoots by a hundred.
  it("shows what a raw comma does to displayToBase", () => {
    expect(BigInt(displayToBase("1000,10", DECIMALS))).toBe(100010n * ONE);
  });

  // The fix: parse first, hand on only the canonical string.
  it.each(["1000.10", " 1000.10 "])("settles %s correctly once parsed", (typed) => {
    const parsed = parseAmount(typed);
    expect(parsed.ok).toBe(true);
    if (!parsed.ok) return;
    expect(BigInt(displayToBase(parsed.canonical, DECIMALS))).toBe(1000100000000000000000n);
  });

  // And the shapes that mis-scale never get there at all.
  it.each(["1000,10", "1.000,10", "1,000.10"])("never lets %s reach displayToBase", (typed) => {
    expect(parseAmount(typed).ok).toBe(false);
  });
});
