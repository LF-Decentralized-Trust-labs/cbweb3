// SPDX-License-Identifier: Apache-2.0

// The shared component package was the one workspace nothing linted: its lint script echoed that
// none was configured, so "eslint clean on the touched files" was true of the apps and vacuous for
// anything living here — including login-errors.ts, which five portals depend on. Review of the
// Scenario B twin (#188) caught that.
//
// Same config the apps use, so this package is held to the rules its consumers are.
import reactConfig from "@cbweb3/config/eslint/react";

export default [
  ...reactConfig,
  {
    // Three pre-existing violations, enumerated rather than hidden behind an ignored directory, so
    // that every OTHER file — including any component added later — is linted from today:
    //
    //   badge.tsx, button.tsx  react-refresh/only-export-components — these export their `cva`
    //                          variants next to the component, which is how shadcn generates them
    //                          and how the apps import them. Silencing it needs the variants moved
    //                          to their own files.
    //   chart.tsx              @typescript-eslint/no-explicit-any.
    //
    // Fixing them is a component change and does not belong in a PR about login error messages.
    // Deleting an entry here is how they get closed, one file at a time.
    ignores: ["src/components/badge.tsx", "src/components/button.tsx", "src/components/chart.tsx"],
  },
];
