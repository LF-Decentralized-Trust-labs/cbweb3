// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import {
  AGREEMENT_AMOUNT_DECIMALS,
  agreementAmountToInput,
  agreementInputToBase,
  formatAgreementAmount,
} from "../agreement-amounts";

// FX agreement amounts are base-unit integers in the application scale (hundredths) on both
// legs. These helpers are the one place they are converted, so every page shows the same figure
// and a pre-filled form submits exactly the agreement's amount.

describe("agreement amount conversion", () => {
  it("uses the application scale the orchestrator uses", () => {
    expect(AGREEMENT_AMOUNT_DECIMALS).toBe(2);
  });

  it("round-trips the agreement's figure exactly", () => {
    for (const base of ["1000000", "100010", "1", "0", "123456789012345678901234567890"]) {
      expect(agreementInputToBase(agreementAmountToInput(base))).toBe(base);
    }
  });

  it("pre-fills what an operator would type, at full precision", () => {
    expect(agreementAmountToInput("1000000")).toBe("10000");
    expect(agreementAmountToInput("100010")).toBe("1000.1");
    expect(agreementAmountToInput("5")).toBe("0.05");
  });

  it("does not depend on when the token decimals load", () => {
    // The defect this replaces: converted at the default scale, submitted at the loaded one.
    const typed = agreementAmountToInput("1000000");
    expect(agreementInputToBase(typed)).toBe("1000000");
  });

  it("pre-fills nothing for a missing or malformed amount", () => {
    expect(agreementAmountToInput(undefined)).toBe("");
    expect(agreementAmountToInput("12x")).toBe("");
  });
});

describe("formatAgreementAmount", () => {
  it("shows two-decimal currencies with their minor unit and grouping", () => {
    expect(formatAgreementAmount("1000000", "BRL")).toBe("10,000.00 BRL");
    expect(formatAgreementAmount("100010", "COP")).toBe("1,000.10 COP");
  });

  it("shows zero-decimal currencies without a fraction", () => {
    expect(formatAgreementAmount("100000", "CLP")).toBe("1,000 CLP");
    expect(formatAgreementAmount("250000", "PYG")).toBe("2,500 PYG");
  });

  it("never hides money: digits beyond the minor unit are shown, not truncated", () => {
    expect(formatAgreementAmount("100050", "CLP")).toBe("1,000.5 CLP");
    expect(formatAgreementAmount("1", "BRL")).toBe("0.01 BRL");
  });

  it("falls back to the agreement scale for an unknown currency", () => {
    expect(formatAgreementAmount("1000", "XYZ")).toBe("10.00 XYZ");
  });

  it("marks a missing amount instead of showing zero", () => {
    expect(formatAgreementAmount(undefined, "BRL")).toBe("— BRL");
  });
});
