// SPDX-License-Identifier: Apache-2.0

// Guards the confirmation step on every swap flow of the bank portal (R2-M-8).
//
// All of them used to execute straight off the form submit: one click settled
// on-chain, with no step showing the operator the amounts about to be sent.
//
// Which page is actually reachable is not obvious and was got wrong once. The
// route list is split by build flag in routes/index.tsx: `{path: "amm"}` belongs
// to scenarioAChildren, so with VITE_SCENARIO=scenario-b (what the toolkit
// passes) AMMTradingPage is NOT routed and Vite drops it from the bundle —
// verified by grepping the built asset of a deployed portal. The reachable swap
// screen of scenario B is CrossCurrencyBridgePage, at `{path: "bridge"}`.
// Both are covered here so neither regresses.
//
// This is a source-level guard rather than a rendering test: the repository has
// no DOM test harness (no @testing-library anywhere in either monorepo) and
// adding one for this would be a new dependency. The same approach is used for
// the Dockerfile hardening guards in the toolkits. What it pins is the shape
// that matters: a submit handler must not perform the swap itself.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const source = readFileSync(fileURLToPath(new URL("../AMMTradingPage.tsx", import.meta.url)), "utf8");
const bridgeSource = readFileSync(fileURLToPath(new URL("../CrossCurrencyBridgePage.tsx", import.meta.url)), "utf8");
const routes = readFileSync(fileURLToPath(new URL("../../routes/index.tsx", import.meta.url)), "utf8");

/** Extracts the body of every arrow function bound to a `handle*` const. */
function handlerBodies(text: string): { name: string; body: string }[] {
  const out: { name: string; body: string }[] = [];
  const re = /const (handle[A-Za-z]*)\s*=\s*(?:async\s*)?\([^)]*\)\s*(?::[^=]*)?=>\s*\{/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(text)) !== null) {
    // Walk braces from the opening one to find the matching close.
    let depth = 0;
    let i = re.lastIndex - 1;
    for (; i < text.length; i += 1) {
      if (text[i] === "{") depth += 1;
      else if (text[i] === "}") {
        depth -= 1;
        if (depth === 0) break;
      }
    }
    out.push({ name: match[1], body: text.slice(re.lastIndex, i) });
  }
  return out;
}

describe("AMM swap confirmation", () => {
  it("renders a confirmation dialog for both swap flows", () => {
    // One for the direct pool swap, one for the cross-currency swap.
    const occurrences = source.match(/<ConfirmActionDialog/g) ?? [];
    expect(occurrences).toHaveLength(2);
  });

  it("no submit handler executes a swap directly", () => {
    const offenders = handlerBodies(source)
      .filter((handler) => /executeSwap\s*\(/.test(handler.body))
      .map((handler) => handler.name);
    expect(offenders).toEqual([]);
  });

  it("both submit handlers open the confirmation instead", () => {
    const opening = handlerBodies(source).filter((handler) => /setConfirmingSwap\(true\)/.test(handler.body));
    expect(opening.map((handler) => handler.name).sort()).toEqual(["handleExecuteSwap", "handleSwap"]);
  });

  it("the swap is still reachable from a confirm callback", () => {
    // The guard above must not be satisfiable by deleting the swap entirely.
    expect(/const confirmSwap = async \(\) => \{/.test(source)).toBe(true);
    expect(/const confirmExecuteSwap = async \(\) => \{/.test(source)).toBe(true);
    expect((source.match(/await executeSwap\(/g) ?? []).length).toBe(2);
  });
});

describe("cross-currency bridge confirmation (the reachable scenario B screen)", () => {
  it("renders a confirmation dialog", () => {
    expect((bridgeSource.match(/<ConfirmActionDialog/g) ?? []).length).toBe(1);
  });

  it("the submit handler does not execute the swap directly", () => {
    const offenders = handlerBodies(bridgeSource)
      .filter((handler) => /executeSwap\s*\(/.test(handler.body))
      .map((handler) => handler.name);
    expect(offenders).toEqual([]);
  });

  it("the submit handler opens the confirmation", () => {
    const opening = handlerBodies(bridgeSource).filter((h) => /setConfirmingSwap\(true\)/.test(h.body));
    expect(opening.map((h) => h.name)).toEqual(["handleExecuteSwap"]);
  });

  it("the swap is still reachable from the confirm callback", () => {
    expect(/const confirmExecuteSwap = async \(\) => \{/.test(bridgeSource)).toBe(true);
    expect((bridgeSource.match(/await executeSwap\(/g) ?? []).length).toBe(1);
  });
});

describe("which swap screen the build actually ships", () => {
  it("the bridge page is in the scenario B route set and the AMM page is not", () => {
    // Pins the fact that made the earlier reading wrong: the `amm` route sits in
    // the scenario A list, so a scenario-b build does not ship AMMTradingPage.
    const bStart = routes.indexOf("const scenarioBChildren");
    expect(bStart).toBeGreaterThan(-1);
    const bBlock = routes.slice(bStart, routes.indexOf("];", bStart));
    expect(bBlock).toContain("CrossCurrencyBridgePage");
    expect(bBlock).not.toContain("AMMTradingPage");
  });
});
