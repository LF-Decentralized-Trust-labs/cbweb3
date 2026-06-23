// SPDX-License-Identifier: Apache-2.0

import { useEffect } from "react";
import { Outlet } from "react-router-dom";
import { useCircuitBreaker } from "../../hooks";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";

export function AppLayout() {
  const { circuitBreaker, fetchState } = useCircuitBreaker();

  useEffect(() => {
    void fetchState();
  }, [fetchState]);

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
        <main className="flex-1 overflow-y-auto p-4">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
