// SPDX-License-Identifier: Apache-2.0

// No API error in this portal may be surfaced as axios's own message.
//
// `error.message` on a rejected axios request is "Request failed with status code NNN". The
// gateway's explanation — which for 216 of its handlers is prose in `response.data.error` — is
// thrown away. Before this guard the pattern appeared 34 times across 16 files, so every failure
// in the portal, whatever its cause, read the same to the operator.
//
// Guarded at the source level rather than store by store: the defect is a one-line habit that
// spreads by copy-paste into the next `catch` block, and a behavioural test only ever covers the
// stores someone remembered to write one for. The wiring of one real store is proved separately in
// stores/__tests__/statement.store.error.test.ts; this is what covers the other thirty-three.
//
// Same shape as pages/__tests__/confirmation-amount-units.test.ts, and for the same reason: this
// monorepo has no DOM harness, and adding one for a lint-shaped rule would be a new dependency.
//
// Stated bound, so no reader assumes more than this does: it is a regex over source text. Idioms
// that mean the same thing but do not look like the one below — `(error as Error)?.message ??
// fallback` is the obvious one — pass it. The scan holds a habit down; it does not prove absence.

import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const srcDir = fileURLToPath(new URL("..", import.meta.url));

/**
 * The reverted form: any variable's own `.message` used as the operator-facing text of a failure.
 * Matched on the ternary rather than on `.message` alone, because `.message` is legitimate inside
 * the helper's own callers and in log lines.
 *
 * Matched against `collapsed()` source, never the file as written. Prettier wraps this ternary
 * across three lines as soon as it passes printWidth, and the longer the fallback message the more
 * likely that is — so a scan that only saw the one-line form would be defeated by the repo's own
 * formatter, with no hand-crafting required. It is not a hypothetical: RedeemsPage.tsx carried the
 * wrapped form through the first draft of this branch, which is why develop's count is 34 across 16
 * files and not the 33 across 15 a single-line scan reports.
 */
const AXIOS_MESSAGE_FALLBACK = /(\w+) instanceof Error \? \1\.message/;

/** Whitespace-insensitive source, so formatting cannot decide whether a rule applies. */
const collapsed = (source: string) => source.replace(/\s+/g, " ");

const CATCHES_A_FAILURE = /catch \(\w+\)/;

/**
 * A file handles an API failure if it catches and reaches the API layer.
 *
 * Matched on the import path, not on the shape of the call. The first version of this guard tested
 * `/\bapi\w*\.\w+\(/i`, which cannot match `statementApi.list(` — `\b` wants a word boundary before
 * `api` and there is none between `t` and `A` — and every API object in this portal is named that
 * way. It selected zero files, so `expect([]).toEqual([])` passed on every input, including a tree
 * with the helper stripped out entirely. The import path is what the layer *is*; the call shape is
 * a naming convention that a rename would quietly break again.
 */
const REACHES_API_LAYER = /from ["'][^"']*services\/api/;

/**
 * Nine files match today; the floor sits one below, so deleting a store is a change and not a
 * failure. It exists because the way this assertion broke was not a wrong answer but an empty
 * question: a predicate that stops matching drops straight to zero, and `expect([]).toEqual([])`
 * then passes on every tree there is. An empty offender list is evidence only if the filter that
 * produced it looked at something.
 */
const MIN_API_FAILURE_HANDLERS = 8;

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
      .filter(({ source }) => AXIOS_MESSAGE_FALLBACK.test(collapsed(source)))
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
    const handlers = files.filter(
      ({ source }) => CATCHES_A_FAILURE.test(source) && REACHES_API_LAYER.test(source),
    );

    expect(
      handlers.length,
      `the predicate selected ${handlers.length} files, below the floor of ` +
        `${MIN_API_FAILURE_HANDLERS}. Either the API layer moved out of services/api — update ` +
        `REACHES_API_LAYER — or this assertion is now watching nothing, which is how it failed ` +
        `before.`,
    ).toBeGreaterThanOrEqual(MIN_API_FAILURE_HANDLERS);

    const missing = handlers
      .filter(({ source }) => !EXPLAINS.some((helper) => source.includes(helper)))
      .map(({ path }) => path);

    expect(
      missing,
      `these files catch an API failure without apiErrorMessage or loginErrorMessage:\n  ` +
        missing.join("\n  "),
    ).toEqual([]);
  });
});
