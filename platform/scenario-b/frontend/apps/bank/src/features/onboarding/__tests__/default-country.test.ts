// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { defaultCountryForCurrency } from "../default-country";

// The onboarding wizard used to present "BR" everywhere, so an Argentine bank was one
// unnoticed field away from being registered in the wrong jurisdiction. These tests pin
// the two halves of the fix: the portal derives its own country, and it refuses to
// guess when it cannot.

describe("defaultCountryForCurrency", () => {
  it("derives the country from the spoke's sovereign currency", () => {
    expect(defaultCountryForCurrency("BRL")).toBe("BR");
    expect(defaultCountryForCurrency("ARS")).toBe("AR");
    expect(defaultCountryForCurrency("COP")).toBe("CO");
  });

  it("does not hand an Argentine portal a Brazilian country", () => {
    // The reported defect, stated as a test: this must never be "BR".
    expect(defaultCountryForCurrency("ARS")).not.toBe("BR");
  });

  it("returns empty for a currency it does not know, rather than guessing", () => {
    // An empty field makes the operator choose. A wrong one is accepted in silence,
    // which is exactly how the original default survived review.
    expect(defaultCountryForCurrency("XYZ")).toBe("");
  });

  it("returns empty when the symbol is missing or blank", () => {
    expect(defaultCountryForCurrency(undefined)).toBe("");
    expect(defaultCountryForCurrency(null)).toBe("");
    expect(defaultCountryForCurrency("   ")).toBe("");
  });

  it("tolerates the casing and padding of a build-time environment value", () => {
    expect(defaultCountryForCurrency(" ars ")).toBe("AR");
    expect(defaultCountryForCurrency("brl")).toBe("BR");
  });
});
