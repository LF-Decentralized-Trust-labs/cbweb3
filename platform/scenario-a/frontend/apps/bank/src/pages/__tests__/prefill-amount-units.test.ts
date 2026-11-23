// SPDX-License-Identifier: Apache-2.0

// An FX agreement amount must be converted before anything uses it.
//
// The API returns FX agreement amounts (`origin_amount`, `counter_amount`) as base-unit integers.
// The HTLC settlement form once pre-filled its amount field straight from the agreement, so submit
// scaled an already-scaled value: at 2 decimals an agreement of 1000000 (10,000.00) locked
// 100000000, and the orchestrator refused the lock with
//
//   HTLC amount "100000000" does not match FX agreement amount "1000000"
//
// Other pages showed the same raw integers to the user. All conversions now live in
// features/fx/agreement-amounts.ts. This guard checks, at the source because this app has no DOM
// test harness, that every READ of an agreement amount anywhere in src/ goes straight into one of
// those helpers. Writing the fields (building a proposal request) is not a read and is not flagged.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const srcDir = fileURLToPath(new URL("../..", import.meta.url));

/** The agreement fields that carry an amount in base units. */
const AGREEMENT_AMOUNT_FIELDS = ["origin_amount", "counter_amount"];

/** The helpers that may receive an agreement amount directly. */
const CONVERTERS = ["agreementAmountToInput", "formatAgreementAmount"];

/** The module that defines the converters, and the type declarations, are not reads. */
const EXEMPT = [join("features", "fx", "agreement-amounts.ts"), join("types", "")];

/** Every .ts and .tsx file under dir, recursively, except tests. */
function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) {
      if (name !== "__tests__") out.push(...sourceFiles(path));
    } else if (/\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name)) {
      out.push(path);
    }
  }
  return out;
}

/**
 * Reads of an agreement amount that do not go straight into a converter. Covers member access
 * (`ag.origin_amount`, `ag?.origin_amount`) and bracket access (`ag["origin_amount"]`), whatever
 * surrounds it: `?? ""`, an intermediate variable, a template literal, JSX.
 */
export function unconvertedReads(rawSource: string): string[] {
  // Comments are prose, not reads. Blanked rather than removed, so the line shown for a hit
  // still matches the file. The (^|[^:]) keeps "http://" in a string from being taken as one.
  const source = rawSource.replace(/\/\*[\s\S]*?\*\/|(^|[^:])\/\/.*$/gm, (c, lead: string | undefined) =>
    (lead ?? "") + c.slice((lead ?? "").length).replace(/[^\n]/g, " "),
  );
  const fields = AGREEMENT_AMOUNT_FIELDS.join("|");
  // No whitespace between the dot and the name: a sentence ending in "." followed by a
  // request key on the next line is not a member access.
  const read = new RegExp(`(?:\\?\\.|\\.)(?:${fields})\\b|\\[\\s*["'\`](?:${fields})["'\`]\\s*\\]`, "g");
  const hits: string[] = [];
  for (const m of source.matchAll(read)) {
    // Walk back over the object expression (identifiers, dots, optional chaining, brackets).
    let start = m.index ?? 0;
    while (start > 0 && /[\w$.?\]["'`]/.test(source[start - 1])) start--;
    const before = source.slice(0, start).trimEnd();
    if (CONVERTERS.some((fn) => new RegExp(`\\b${fn}\\($`).test(before))) continue;
    const lineStart = source.lastIndexOf("\n", m.index) + 1;
    const lineEnd = source.indexOf("\n", m.index);
    hits.push(source.slice(lineStart, lineEnd < 0 ? undefined : lineEnd).trim());
  }
  return hits;
}

describe("agreement amounts are always converted", () => {
  const files = sourceFiles(srcDir);

  it("scans all of src/, recursively, .ts and .tsx", () => {
    const rel = files.map((f) => relative(srcDir, f));
    expect(rel).toContain(join("pages", "HTLCNewPage.tsx"));
    expect(rel.some((f) => f.split("/").length > 2 && f.endsWith(".ts"))).toBe(true);
  });

  it("never reads origin_amount or counter_amount except into a converter", () => {
    const offenders: string[] = [];
    for (const file of files) {
      const rel = relative(srcDir, file);
      if (EXEMPT.some((e) => rel === e || (e.endsWith("/") && rel.startsWith(e)))) continue;
      for (const hit of unconvertedReads(readFileSync(file, "utf8"))) {
        offenders.push(`${rel}: ${hit} — use agreementAmountToInput or formatAgreementAmount`);
      }
    }
    expect(offenders).toEqual([]);
  });

  // The guard is worth nothing if it cannot see the mistake, so this proves it sees each variant
  // of it, and that the converted forms pass.
  it("detects every variant of a raw read", () => {
    for (const bad of [
      "setLockAmount(ag.origin_amount);",
      "setHashAmount( selected?.counter_amount );",
      'setLockAmount(ag.origin_amount ?? "");',
      'setLockAmount(ag["origin_amount"]);',
      "const amt = ag.origin_amount; setLockAmount(amt);",
      "<span>{a.counter_amount} {a.counter_currency}</span>",
      "`amount = ${ag.origin_amount}`",
      "setLockAmount(baseToExactDisplay(ag.origin_amount, tCeBMDecimals));",
    ]) {
      expect(unconvertedReads(bad), bad).toHaveLength(1);
    }
  });

  it("accepts converted reads and request writes", () => {
    for (const good of [
      "setLockAmount(agreementAmountToInput(ag.origin_amount));",
      'setLockAmount(agreementAmountToInput(ag.origin_amount ?? ""));',
      "{formatAgreementAmount(a.counter_amount, a.counter_currency)}",
      'formatAgreementAmount(ag["origin_amount"], "BRL")',
      "origin_amount: agreementInputToBase(parsedOrigin.canonical),",
      "type FX = { origin_amount: string };",
      "// the relay copies req.origin_amount verbatim",
      "/* ag.counter_amount is base units */",
      "const req = { note: x.\n  origin_amount: agreementInputToBase(v) };",
    ]) {
      expect(unconvertedReads(good), good).toEqual([]);
    }
  });
});
