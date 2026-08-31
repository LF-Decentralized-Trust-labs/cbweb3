// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from "vitest";

// isScenarioB gates the Scenario B route set, the sidebar and — since the breaker
// indicator was re-sourced from on-chain per-pair status — the chrome's status polling.
// Vite folds the comparison at build time, so a spelling the flag does not recognise
// removes those code paths from the bundle silently: nothing throws, the screens are
// simply absent and the indicator never fetches. This pins both spellings the build
// environments actually emit — the toolkit passes "scenario-b"
// (toolkit/engine/orchestrator/step_found_spoke.go), docker-compose.scenario-b.yml
// passes "b".
async function loadFlag(value: string | undefined): Promise<boolean> {
  vi.resetModules();
  if (value === undefined) {
    vi.stubEnv("VITE_SCENARIO", "");
  } else {
    vi.stubEnv("VITE_SCENARIO", value);
  }
  const mod = await import("./scenario");
  return mod.isScenarioB;
}

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

describe("isScenarioB", () => {
  it('accepts the toolkit spelling "scenario-b"', async () => {
    await expect(loadFlag("scenario-b")).resolves.toBe(true);
  });

  it('accepts the compose spelling "b"', async () => {
    await expect(loadFlag("b")).resolves.toBe(true);
  });

  it("is false for Scenario A", async () => {
    await expect(loadFlag("a")).resolves.toBe(false);
    await expect(loadFlag("scenario-a")).resolves.toBe(false);
  });

  it("is false when the build environment provides nothing", async () => {
    await expect(loadFlag(undefined)).resolves.toBe(false);
  });
});
