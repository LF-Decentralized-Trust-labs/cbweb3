// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { useUiStore } from "../../stores";
import { isDataStale, useFreshnessStore } from "../../stores/freshness.store";
import { StaleDataBanner } from "../common/StaleDataBanner";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";
import { TopStatusBar } from "./TopStatusBar";

// The telemetry WebSocket is not implemented: services/websocket/telemetry.service.ts is
// a mock frame generator, so connecting it would only fabricate a "connected" state. The
// portal is HTTP polling end to end, and staleness is measured from the last successful
// backend call (see stores/freshness.store.ts). The mock is left in place, unwired, for
// whenever a real stream lands.
const TICK_MS = 2000;

export function AppLayout() {
  const pollingSeconds = useUiStore((state) => state.fallbackPollingSeconds);
  const lastSuccessAt = useFreshnessStore((state) => state.lastSuccessAt);
  const lastErrorAt = useFreshnessStore((state) => state.lastErrorAt);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  const stale = isDataStale(lastSuccessAt, now, pollingSeconds, lastErrorAt);

  return (
    <div className="flex min-h-screen flex-col bg-muted/30">
      <TopStatusBar />
      <Header />
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col md:flex-row">
        <Sidebar />
        <main className="min-w-0 flex-1 p-4">
          <StaleDataBanner stale={stale} />
          <Outlet />
        </main>
      </div>
    </div>
  );
}
