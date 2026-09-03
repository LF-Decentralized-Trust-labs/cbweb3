// SPDX-License-Identifier: Apache-2.0

import { Outlet } from "react-router-dom";
import { isScenarioB } from "../../config/scenario";
import { useCircuitBreaker } from "../../hooks";
import { usePolling } from "../../hooks/usePolling";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppLayout() {
  const { circuitBreaker, fetchState } = useCircuitBreaker();

  // Refresh on the same cadence as the Circuit Breaker page, so the chrome banner and the
  // page converge rather than the banner holding a stale claim until the next navigation.
  // This is also what lets an indeterminate condition (pair set unknown) recover by itself.
  // Gated on the scenario for the same reason DashboardPage gates it: the per-pair breaker
  // status is a Scenario B capability.
  usePolling(() => void fetchState(), 15000, isScenarioB);

  // Only an affirmative halt raises the banner. An indeterminate condition (null) never does:
  // the chrome must not assert a stop it cannot substantiate.
  const halted = circuitBreaker?.state === "HALTED";

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-muted/30">
      {halted ? (
        <div className="bg-destructive px-4 py-2 text-center text-xs font-medium text-destructive-foreground">
          Emergency mode active: swaps are globally halted.
        </div>
      ) : null}
      <Header />
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col overflow-hidden md:flex-row">
        <Sidebar />
        <main className="min-w-0 flex-1 overflow-y-auto p-4">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
