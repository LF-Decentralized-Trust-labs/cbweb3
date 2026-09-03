// SPDX-License-Identifier: Apache-2.0

// The toolkit builds pass VITE_SCENARIO="scenario-b"; accept the short "b" too so
// the flag is true regardless of which spelling the build environment provides.
// Mirrors apps/bank/src/config/scenario.ts. Getting this wrong is silent: Vite folds
// the comparison at build time, so the Scenario B routes and the chrome breaker
// indicator are dead-code-eliminated rather than failing loudly.
const scenario = import.meta.env.VITE_SCENARIO;
export const isScenarioB = scenario === "b" || scenario === "scenario-b";
