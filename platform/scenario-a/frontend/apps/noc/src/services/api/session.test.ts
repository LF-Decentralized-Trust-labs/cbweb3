// SPDX-License-Identifier: Apache-2.0

import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// The acceptance criterion this card names first: no token is read from or written to web
// storage, "asserted by a test, not by inspection".
//
// A source scan rather than a runtime stub, and deliberately so. A runtime test proves only
// the paths it exercises; the exposure is any line anywhere that puts a credential where a
// page script can read it, and that is a property of the source. It also survives a
// refactor that moves the code, which a mock of localStorage would not.

const SRC = join(__dirname, "..", "..");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return sourceFiles(full);
    return /\.(ts|tsx)$/.test(entry) && !/\.test\.tsx?$/.test(entry) ? [full] : [];
  });
}

describe("the NOC portal keeps no session in web storage", () => {
  it("never touches localStorage or sessionStorage for a token", () => {
    const offenders: string[] = [];

    for (const file of sourceFiles(SRC)) {
      const text = readFileSync(file, "utf8");
      let inBlockComment = false;
      text.split("\n").forEach((line, i) => {
        // Comments are skipped, and that is not a loophole: the rule is about what the code
        // DOES. Matching prose would flag the comment in session.ts that explains why this
        // rule exists, which is the opposite of useful.
        const trimmed = line.trim();
        if (inBlockComment) {
          if (trimmed.includes("*/")) inBlockComment = false;
          return;
        }
        if (trimmed.startsWith("/*")) {
          if (!trimmed.includes("*/")) inBlockComment = true;
          return;
        }
        if (trimmed.startsWith("//") || trimmed.startsWith("*")) return;
        const code = line.split("//")[0];
        if (!/\b(localStorage|sessionStorage)\b/.test(code)) return;
        // The UI settings store persists display preferences — a collapsed panel, a chosen
        // refresh interval. Those are not credentials, and forbidding all web storage would
        // be a rule nobody could follow.
        if (/ui[.-]?settings|preferences|theme/i.test(file)) return;
        offenders.push(`${file.replace(SRC, "src")}:${i + 1}: ${line.trim()}`);
      });
    }

    expect(
      offenders,
      "web storage is used outside the settings store. The session lives in an HttpOnly "
        + "cookie precisely so no page script can read it; putting a token back into storage "
        + "reopens the exposure this change closed:\n" + offenders.join("\n"),
    ).toEqual([]);
  });

  it("does not call Keycloak's token endpoint from the browser", () => {
    // The grant moved to the backend, because only a server can set a cookie the browser
    // cannot read. A direct call from here would mean the tokens came back into JavaScript.
    const offenders = sourceFiles(SRC).filter((file) =>
      /openid-connect\/token/.test(readFileSync(file, "utf8")),
    );

    expect(
      offenders.map((f) => f.replace(SRC, "src")),
      "the portal is calling Keycloak directly again; the login must go through the NOC backend",
    ).toEqual([]);
  });
});
