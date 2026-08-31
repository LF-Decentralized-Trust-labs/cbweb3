// SPDX-License-Identifier: Apache-2.0

import { Badge } from "@cbweb3/ui";
import { useAlertStore, useWebsocketStore } from "../../stores";

export function TopStatusBar() {
  const isConnected = useWebsocketStore((state) => state.isConnected);
  const stale = useWebsocketStore((state) => state.stale);
  const criticalCount = useAlertStore((state) => state.alerts.filter((alert) => alert.severity === "CRITICAL").length);

  return (
    <div className="border-b border-border bg-background/70 px-4 py-2 text-xs">
      <div className="mx-auto flex max-w-7xl items-center gap-2">
        <Badge variant={isConnected ? "success" : "destructive"}>{isConnected ? "WS CONNECTED" : "WS DISCONNECTED"}</Badge>
        <Badge variant={stale ? "warning" : "secondary"}>{stale ? "STALE DATA" : "LIVE DATA"}</Badge>
        <Badge variant={criticalCount > 0 ? "destructive" : "outline"}>CRITICAL ALERTS: {criticalCount}</Badge>
      </div>
    </div>
  );
}
