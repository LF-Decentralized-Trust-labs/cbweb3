// SPDX-License-Identifier: Apache-2.0

// An amount read from the API must be converted before it is put into a form.
//
// The API returns FX agreement amounts (`origin_amount`, `counter_amount`) as base-unit
// integers. A form's amount state holds what an operator would TYPE, and the form scales it
// with displayToBase on submit. The HTLC settlement form pre-filled its amount straight from
// the agreement, so submit scaled an already-scaled value: at 2 decimals an agreement of
// 1000000 (10,000.00) locked 100000000, and the orchestrator refused the lock with
//
//   HTLC amount "100000000" does not match FX agreement amount "1000000"
//
// Every pre-filled settlement from an agreement failed this way. Like the confirmation-panel
// guard next to it, this is checked at the source: this app has no DOM test harness.

import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const pagesDir = fileURLToPath(new URL("..", import.meta.url));

/** API fields that carry an amount in BASE units. */
const BASE_UNIT_FIELDS = ["origin_amount", "counter_amount", "amount"];

/** A setter of amount state handed a base-unit field directly, e.g. setLockAmount(ag.origin_amount). */
function rawPrefills(source: string): string[] {
  const fields = BASE_UNIT_FIELDS.join("|");
  const call = new RegExp(`\\bset\\w*[Aa]mount\\(\\s*[\\w.?]+\\.(?:${fields})\\s*\\)`, "g");
  return source.match(call) ?? [];
}

describe("amount fields are pre-filled in display units", () => {
  it("never puts an API base-unit amount straight into a form's amount state", () => {
    const offenders: string[] = [];
    for (const name of readdirSync(pagesDir).filter((f) => f.endsWith(".tsx"))) {
      const source = readFileSync(`${pagesDir}/${name}`, "utf8");
      for (const hit of rawPrefills(source)) {
        offenders.push(`${name}: ${hit} — convert with baseToExactDisplay first`);
      }
    }
    expect(offenders).toEqual([]);
  });

  // The guard is worth nothing if it cannot see the mistake, so this proves it can, and that
  // the converted form passes.
  it("detects a raw pre-fill and accepts a converted one", () => {
    expect(rawPrefills("setLockAmount(ag.origin_amount);")).toHaveLength(1);
    expect(rawPrefills("setHashAmount( selected?.counter_amount );")).toHaveLength(1);
    expect(rawPrefills("setLockAmount(baseToExactDisplay(ag.origin_amount, tCeBMDecimals));")).toHaveLength(0);
  });
});
