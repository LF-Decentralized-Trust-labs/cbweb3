// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  baseToExactDisplay,
  displayToBase,
  formatBaseUnits,
  formatCeBM,
  formatCeBMDisplay,
  formatFiatUnits,
} from "../payment.types";

const DEC = 18;

// Before ADR-009 these printed the raw integer, so 1000 wei of an 18-decimal token
// read as "1,000 fCeBM" — the right number under the wrong unit.
describe("formatBaseUnits", () => {
  it("renders the currency's minor units", () => {
    expect(formatBaseUnits("100200000000000000000", DEC)).toBe("100.20");
    expect(formatBaseUnits("1000000000000000000", DEC)).toBe("1.00");
    expect(formatBaseUnits("0", DEC)).toBe("0.00");
  });

  // A balance must never read as more than it is. An AMM-style residue truncates down.
  it("truncates rather than rounds", () => {
    expect(formatBaseUnits("3959753632757569189", DEC)).toBe("3.95");
    expect(formatBaseUnits("1999999999999999999", DEC)).toBe("1.99");
  });

  // Dust is still money. Rendering it as "0.00" is the same class of defect as
  // rendering the wrong figure.
  it("never shows a non-zero holding as zero", () => {
    expect(formatBaseUnits("1", DEC)).toBe("< 0.01");
    expect(formatBaseUnits("9999999999999999", DEC)).toBe("< 0.01");
  });

  it("labels the token", () => {
    expect(formatCeBM("100200000000000000000", DEC)).toBe("100.20 tCeBM");
    expect(formatFiatUnits("100200000000000000000", DEC, "fCeBM_BRL")).toBe("100.20 fCeBM_BRL");
  });
});

// Prefills feed an amount input, so they keep every digit: a truncated suggestion is a
// different transaction, not a different label.
describe("baseToExactDisplay", () => {
  it("keeps full precision", () => {
    expect(baseToExactDisplay("3959753632757569189", DEC)).toBe("3.959753632757569189");
    expect(baseToExactDisplay("100200000000000000000", DEC)).toBe("100.2");
    expect(baseToExactDisplay("1000000000000000000", DEC)).toBe("1");
  });
});

describe("displayToBase", () => {
  it("scales what the operator typed", () => {
    expect(displayToBase("100.20", DEC)).toBe("100200000000000000000");
    expect(displayToBase("1", DEC)).toBe("1000000000000000000");
    expect(displayToBase("0.000000000000000001", DEC)).toBe("1");
  });

  // A round trip through the two must not move the figure.
  it.each(["100.20", "1", "0.5", "3.959753632757569189"])("round-trips %s", (typed) => {
    expect(baseToExactDisplay(displayToBase(typed, DEC), DEC)).toBe(
      typed.includes(".") ? typed.replace(/0+$/, "").replace(/\.$/, "") : typed,
    );
  });
});

// The pairing that shipped the "0 BRL" confirmation in the other scenario: a form's
// typed value handed to a base-unit formatter, which divides it a second time.
describe("display values must be converted before formatting", () => {
  it("divides twice if the display formatter is skipped", () => {
    // What a careless call site does: the form holds "1500", the operator sees this.
    expect(formatCeBM("1500", DEC)).toBe("< 0.01 tCeBM");
    // What it must do instead.
    expect(formatCeBMDisplay("1500", DEC)).toBe("1,500.00 tCeBM");
  });
});
