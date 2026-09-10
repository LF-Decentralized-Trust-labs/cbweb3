// SPDX-License-Identifier: Apache-2.0

// No API error in this portal may be surfaced as axios's own message.
//
// `error.message` on a rejected axios request is "Request failed with status code NNN". The
// gateway's explanation — which for 214 of its handlers is prose in `response.data.error` — is
// thrown away. Before this guard the pattern appeared 33 times across 15 files, so every failure
// in the portal, whatever its cause, read the same to the operator.
//
// Guarded at the source level rather than store by store: the defect is a one-line habit that
// spreads by copy-paste into the next `catch` block, and a behavioural test only ever covers the
// stores someone remembered to write one for. The wiring of one real store is proved separately in
// stores/__tests__/statement.store.error.test.ts; this is what covers the other thirty-two.
//
// Same shape as pages/__tests__/confirmation-amount-units.test.ts, and for the same reason: this
// monorepo has no DOM harness, and adding one for a lint-shaped rule would be a new dependency.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const srcDir = fileURLToPath(new URL("..", import.meta.url));

/**
 * The reverted form: any variable's own `.message` used as the operator-facing text of a failure.
 * Matched on the ternary rather than on `.message` alone, because `.message` is legitimate inside
 * the helper's own callers and in log lines.
 */
const AXIOS_MESSAGE_FALLBACK = /(\w+) instanceof Error \? \1\.message/;

function sourceFiles(dir: string, acc: { path: string; source: string }[] = []) {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "__tests__") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      sourceFiles(full, acc);
    } else if (entry.endsWith(".ts") || entry.endsWith(".tsx")) {
      acc.push({ path: full.slice(srcDir.length), source: readFileSync(full, "utf8") });
    }
  }
  return acc;
}

describe("API errors reach the operator with the gateway's words", () => {
  const files = sourceFiles(srcDir);

  // A guard that silently scanned nothing would report success forever — the same vacuity the
  // licence gate once had.
  it("has sources to scan", () => {
    expect(files.length).toBeGreaterThan(20);
  });

  it("never falls back to the HTTP client's own message", () => {
    const offenders = files
      .filter(({ source }) => AXIOS_MESSAGE_FALLBACK.test(source))
      .map(({ path }) => path);

    expect(
      offenders,
      `these files surface axios's "Request failed with status code NNN" instead of what the ` +
        `gateway said. Use apiErrorMessage(error, "<what failed>") from @cbweb3/ui:\n  ` +
        offenders.join("\n  "),
    ).toEqual([]);
  });

  // The other half: a helper has to actually be reached. A file that catches an API failure and
  // renders a bare fallback passes the check above while telling the operator just as little.
  //
  // Either helper satisfies it. auth.store.ts uses loginErrorMessage, which maps the auth routes'
  // `code` to curated copy — the right choice there and not a gap. The rule is that a caught API
  // failure is explained by one of the two, never by the client's own message or by nothing.
  it("every file that handles a caught API failure explains it with one of the helpers", () => {
    const EXPLAINS = ["apiErrorMessage", "loginErrorMessage"];
    const missing = files
      .filter(({ source }) => /catch \(\w+\)/.test(source) && /\bapi\w*\.\w+\(/i.test(source))
      .filter(({ source }) => !EXPLAINS.some((helper) => source.includes(helper)))
      .map(({ path }) => path);

    expect(
      missing,
      `these files catch an API failure without apiErrorMessage or loginErrorMessage:\n  ` +
        missing.join("\n  "),
    ).toEqual([]);
  });
});
