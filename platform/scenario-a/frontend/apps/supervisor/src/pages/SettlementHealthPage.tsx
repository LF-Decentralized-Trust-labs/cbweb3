// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";
import { useEffect } from "react";
import { useStabilityStore } from "../stores";

const badgeFromState = (state: string): "default" | "secondary" | "destructive" | "warning" => {
  if (state === "HTLC_STATE_SETTLED") return "default";
  if (state === "HTLC_STATE_LOCKED" || state === "HTLC_STATE_PENDING") return "secondary";
  if (state === "HTLC_STATE_REFUNDED" || state === "HTLC_STATE_INVALID") return "destructive";
  return "warning"; // SETTLING, REFUNDING
};

export function SettlementHealthPage() {
  const { htlcs, alerts, refresh } = useStabilityStore();

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Settlement Health Dashboard</CardTitle>
          <CardDescription>HTLC lifecycle status and timeout risk for active cross-border payments.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-md border border-border p-3">
            <p className="text-sm text-muted-foreground">Active HTLCs</p>
            <p className="mt-1 text-lg font-semibold">{htlcs.filter((h) => h.state === "HTLC_STATE_LOCKED").length}</p>
          </div>
          <div className="rounded-md border border-border p-3">
            <p className="text-sm text-muted-foreground">Active Alerts</p>
            <p className="mt-1 text-lg font-semibold">{alerts.length}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>HTLC Expiry Risk</CardTitle>
          <CardDescription>Contracts approaching timeout require attention before the lock window closes.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {htlcs.map((h) => (
            <div key={h.id} className="flex items-center justify-between rounded-md border border-border p-3 text-sm">
              <div className="space-y-0.5">
                <p className="font-mono text-xs">{h.id.slice(0, 14)}…</p>
                <p className="font-mono text-xs text-muted-foreground">hash: {h.hashLock.slice(0, 14)}…</p>
                <p className="font-mono text-xs text-muted-foreground">zeto: {h.zetoLockRef.slice(0, 14)}…</p>
              </div>
              <div className="flex flex-col items-end gap-1">
                <Badge variant={badgeFromState(h.state)}>{h.state}</Badge>
                <span className="text-xs text-muted-foreground">Expires {new Date(h.expiresAt).toLocaleString()}</span>
              </div>
            </div>
          ))}
          {!htlcs.length ? <p className="text-sm text-muted-foreground">No active HTLCs to monitor.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
