// SPDX-License-Identifier: Apache-2.0

// The toolkit builds pass VITE_SCENARIO="scenario-b"; accept the short "b" too so
// the flag is true regardless of which spelling the build environment provides.
const scenario = import.meta.env.VITE_SCENARIO;
export const isScenarioB = scenario === "b" || scenario === "scenario-b";
