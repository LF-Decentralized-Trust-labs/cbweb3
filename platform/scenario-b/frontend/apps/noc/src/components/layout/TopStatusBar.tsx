// SPDX-License-Identifier: Apache-2.0

import { Badge } from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useAlertStore, useUiStore } from "../../stores";
import { isDataStale, useFreshnessStore } from "../../stores/freshness.store";

/** Re-render cadence for the freshness badge, so it flips even while requests fail. */
const TICK_MS = 2000;

export function TopStatusBar() {
  const pollingSeconds = useUiStore((state) => state.fallbackPollingSeconds);
  const lastSuccessAt = useFreshnessStore((state) => state.lastSuccessAt);
  const lastErrorAt = useFreshnessStore((state) => state.lastErrorAt);
  const criticalCount = useAlertStore((state) => state.alerts.filter((alert) => alert.severity === "CRITICAL").length);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  const stale = isDataStale(lastSuccessAt, now, pollingSeconds, lastErrorAt);

  return (
    <div className="border-b border-border bg-background/70 px-4 py-2 text-xs">
      <div className="mx-auto flex max-w-7xl items-center gap-2">
        <Badge variant="secondary">POLLING {pollingSeconds}s</Badge>
        <Badge variant={stale ? "warning" : "success"}>{stale ? "STALE DATA" : "LIVE DATA"}</Badge>
        <Badge variant={criticalCount > 0 ? "destructive" : "outline"}>CRITICAL ALERTS: {criticalCount}</Badge>
      </div>
    </div>
  );
}
