// SPDX-License-Identifier: Apache-2.0

import { Card, CardContent, CardDescription, CardHeader, CardTitle, Progress } from "@cbweb3/ui";
import { useEffect } from "react";
import { useStabilityStore } from "../stores";

export function StabilityControlsPage() {
  const { pools, alerts, refresh } = useStabilityStore();

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Stability Insights Dashboard</CardTitle>
          <CardDescription>Read-only scenario-B risk posture for AMM network.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2">
          <div className="rounded-md border border-border p-3">
            <p className="text-sm text-muted-foreground">Imbalanced Pools</p>
            <p className="mt-1 text-lg font-semibold">{pools.filter((pool) => pool.isImbalanced).length}</p>
          </div>
          <div className="rounded-md border border-border p-3">
            <p className="text-sm text-muted-foreground">Active Alerts</p>
            <p className="mt-1 text-lg font-semibold">{alerts.length}</p>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Pool Ratio Risk</CardTitle>
          <CardDescription>Target ratio is 50/50 and escalation starts at 70/30.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {pools.map((pool) => (
            <div key={pool.pair} className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span>{pool.pair}</span>
                <span>{pool.ratioA}/{pool.ratioB}</span>
              </div>
              <Progress value={pool.ratioA} />
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
