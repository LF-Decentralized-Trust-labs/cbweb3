// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { evaluateExecuteGating, type ExecuteGatingInput } from "../execute-gating";

const ready: ExecuteGatingInput = {
  step: "idle",
  isPoolActive: true,
  hasQuote: true,
  isQuoteExpired: false,
  amountOut: "50",
  amountIn: "60",
  beneficiaryBankId: "bank-macro",
};

describe("evaluateExecuteGating", () => {
  it("enables Execute once pool, quote and every field are present", () => {
    expect(evaluateExecuteGating(ready)).toEqual({ disabled: false, notice: null });
  });

  // The reported defect: the button went flat on expiry and said so only in a hover
  // tooltip, while the store was already able to refresh and retry.
  it("keeps Execute enabled on an expired quote, and says so in visible text", () => {
    const gating = evaluateExecuteGating({ ...ready, isQuoteExpired: true });
    expect(gating.disabled).toBe(false);
    expect(gating.notice).toMatch(/expired/i);
    expect(gating.notice).toMatch(/price it again/i);
  });

  it("says nothing about expiry before any quote exists", () => {
    const gating = evaluateExecuteGating({ ...ready, hasQuote: false, isQuoteExpired: true });
    expect(gating.disabled).toBe(true);
    expect(gating.notice).toBeNull();
  });

  it("stays quiet about the price while a submission is in flight", () => {
    const gating = evaluateExecuteGating({ ...ready, step: "submitting", isQuoteExpired: true });
    expect(gating.disabled).toBe(true);
    expect(gating.notice).toBeNull();
  });

  it("still blocks on the things a refresh cannot fix", () => {
    expect(evaluateExecuteGating({ ...ready, isPoolActive: false }).disabled).toBe(true);
    expect(evaluateExecuteGating({ ...ready, hasQuote: false }).disabled).toBe(true);
    expect(evaluateExecuteGating({ ...ready, amountOut: "  " }).disabled).toBe(true);
    expect(evaluateExecuteGating({ ...ready, amountIn: "" }).disabled).toBe(true);
    expect(evaluateExecuteGating({ ...ready, beneficiaryBankId: " " }).disabled).toBe(true);
  });
});
