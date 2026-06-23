// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from "vitest";
import { calcMaxAmountIn, weiToDisplay } from "../cross-currency-swap.api";

describe("cross-currency-swap utils", () => {
  it("calcMaxAmountIn applies slippage basis points", () => {
    expect(calcMaxAmountIn("1000000000000000000", 1100)).toBe("1100000000000000000");
  });

  it("calcMaxAmountIn uses default slippage when omitted", () => {
    expect(calcMaxAmountIn("1000000000000000000")).toBe("1100000000000000000");
  });

  it("weiToDisplay formats wei using tokenDecimals=18 and displayDecimals=6", () => {
    expect(weiToDisplay("1000000000000000000", 18, 6)).toBe("1");
    expect(weiToDisplay("1500000000000000000", 18, 6)).toBe("1.5");
    expect(weiToDisplay("1000000000000000000", 18)).toBe("1");
  });

  it("weiToDisplay returns em dash for invalid value", () => {
    expect(weiToDisplay("", 18)).toBe("—");
    expect(weiToDisplay("abc", 18)).toBe("—");
  });
});
