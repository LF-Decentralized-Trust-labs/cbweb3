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

// The application scale (ADR-009): an integer amount means hundredths. The formatters
// take the scale as a parameter, so a couple of cases below still exercise 18 to show
// they are generic — Scenario B runs at that scale.
const DEC = 2;
const DEC18 = 18;

// Before ADR-009 these printed the raw integer, so 1000 wei of an 18-decimal token
// read as "1,000 fCeBM" — the right number under the wrong unit.
describe("formatBaseUnits", () => {
  it("renders the currency's minor units", () => {
    expect(formatBaseUnits("10020", DEC)).toBe("100.20");
    expect(formatBaseUnits("100", DEC)).toBe("1.00");
    expect(formatBaseUnits("100200000000000000000", DEC18)).toBe("100.20");
    expect(formatBaseUnits("0", DEC)).toBe("0.00");
  });

  // A balance must never read as more than it is. An AMM-style residue truncates down.
  it("truncates rather than rounds", () => {
    // At hundredths nothing is truncated — the minor unit IS the smallest amount. The
    // rule still matters for a scenario running at a finer scale, so it is pinned there.
    expect(formatBaseUnits("3959753632757569189", DEC18)).toBe("3.95");
    expect(formatBaseUnits("1999999999999999999", DEC18)).toBe("1.99");
  });

  // Dust is still money. Rendering it as "0.00" is the same class of defect as
  // rendering the wrong figure.
  it("never shows a non-zero holding as zero", () => {
    // Unreachable at hundredths, where one base unit already IS one cent. Kept because
    // the formatter is generic and the branch is live at a finer scale.
    expect(formatBaseUnits("1", DEC)).toBe("0.01");
    expect(formatBaseUnits("1", DEC18)).toBe("< 0.01");
  });

  it("labels the token", () => {
    expect(formatCeBM("10020", DEC)).toBe("100.20 tCeBM");
    expect(formatFiatUnits("10020", DEC, "fCeBM_BRL")).toBe("100.20 fCeBM_BRL");
  });
});

// Prefills feed an amount input, so they keep every digit: a truncated suggestion is a
// different transaction, not a different label.
describe("baseToExactDisplay", () => {
  it("keeps full precision", () => {
    expect(baseToExactDisplay("10020", DEC)).toBe("100.2");
    expect(baseToExactDisplay("100", DEC)).toBe("1");
    expect(baseToExactDisplay("3959753632757569189", DEC18)).toBe("3.959753632757569189");
  });
});

describe("displayToBase", () => {
  it("scales what the operator typed", () => {
    expect(displayToBase("100.20", DEC)).toBe("10020");
    expect(displayToBase("1", DEC)).toBe("100");
    expect(displayToBase("0.01", DEC)).toBe("1");
  });

  // A round trip through the two must not move the figure.
  it.each(["100.20", "1", "0.5"])("round-trips %s", (typed) => {
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
    expect(formatCeBM("1500", DEC)).toBe("15.00 tCeBM");
    // What it must do instead.
    expect(formatCeBMDisplay("1500", DEC)).toBe("1,500.00 tCeBM");
  });
});
