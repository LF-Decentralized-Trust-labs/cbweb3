// SPDX-License-Identifier: Apache-2.0

// The confirmation panels on /deposits, /escrows and /redeems announced the wrong amount:
// they passed the value the operator typed — a DISPLAY string held in component state —
// to formatFiatUnits / formatCeBM, which are documented as taking a raw BASE-UNIT amount
// and dividing by 10^decimals. With 18 decimals, typing 1500 produced
//
//     "0 BRL will be submitted for central bank approval."
//
// while the submit path, which converts with displayToBase, sent the correct 1,500. So the
// number the operator was asked to approve was never the number that went out — on the
// three operations that create and destroy money, whose confirmation step exists for
// exactly this reason (R2-M-8).
//
// These pin the display-taking variants that the panels now use. The bug is a unit
// mismatch, so the tests are written around the units rather than around specific strings.

import { describe, expect, it } from "vitest";
import {
  displayToBase,
  formatCeBMDisplay,
  formatFiatDisplayUnits,
  formatFiatUnits,
} from "../payment.types";

const DECIMALS = 18;

// The on-chain fCeBM symbol, in the shape the API returns it: currencyFromTokenSymbol
// takes the part after the last underscore, so a bare "BRL" would fall back to the
// generic "fiat units" label and the assertions would be about the wrong thing.
const FIAT_SYMBOL = "fCeBM_BRL";

describe("formatFiatDisplayUnits", () => {
  it("renders what the operator typed, not zero", () => {
    // The exact failure a live stack showed: 1500 typed, "0 BRL" announced.
    expect(formatFiatDisplayUnits("1500", DECIMALS, FIAT_SYMBOL)).toBe("1,500 BRL");
  });

  it("never renders 0 for a non-zero amount, at any decimals in use", () => {
    for (const decimals of [2, 6, 18]) {
      for (const typed of ["1", "10", "1500", "0.5", "1,000.25"]) {
        const rendered = formatFiatDisplayUnits(typed, decimals, FIAT_SYMBOL);
        expect(rendered, `${typed} @ ${decimals}dp`).not.toMatch(/^0 /);
      }
    }
  });

  it("keeps a fractional amount", () => {
    expect(formatFiatDisplayUnits("0.5", DECIMALS, FIAT_SYMBOL)).toBe("0.5 BRL");
    expect(formatFiatDisplayUnits("1,000.25", DECIMALS, FIAT_SYMBOL)).toBe("1,000.25 BRL");
  });

  it("still renders a genuine zero as zero", () => {
    // The input starts at "0"; the panel must not claim an amount before one is entered.
    expect(formatFiatDisplayUnits("0", DECIMALS, FIAT_SYMBOL)).toBe("0 BRL");
  });

  it("announces exactly what the submit path will send", () => {
    // The property that was broken: the confirmation and the request must agree. The
    // submit converts with displayToBase, so formatting that conversion is the only
    // rendering guaranteed to match it.
    for (const typed of ["1", "1500", "0.5", "1,000.25"]) {
      expect(formatFiatDisplayUnits(typed, DECIMALS, FIAT_SYMBOL)).toBe(
        formatFiatUnits(displayToBase(typed, DECIMALS), DECIMALS, FIAT_SYMBOL),
      );
    }
  });
});

describe("formatCeBMDisplay", () => {
  it("renders what the operator typed, not zero", () => {
    expect(formatCeBMDisplay("1500", DECIMALS, "tCeBM")).toContain("1,500");
    expect(formatCeBMDisplay("1500", DECIMALS, "tCeBM")).not.toMatch(/^0\b/);
  });

  it("still renders a genuine zero as zero", () => {
    expect(formatCeBMDisplay("0", DECIMALS, "tCeBM")).toMatch(/^0\b/);
  });
});

describe("the base-unit variants are unchanged", () => {
  it("formatFiatUnits keeps taking base units", () => {
    // Balances and stored records arrive in base units and must keep formatting the same
    // way; the fix adds variants rather than changing these.
    expect(formatFiatUnits("1500000000000000000000", DECIMALS, FIAT_SYMBOL)).toBe("1,500 BRL");
  });
});
