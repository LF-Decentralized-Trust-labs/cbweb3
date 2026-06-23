// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useNetworkStore, useStabilityStore } from "../stores";
import { auditApi } from "../services/api";
import type { AuditLogEntry } from "../types";

const badgeFromRatio = (imbalanced: boolean): "destructive" | "success" => (imbalanced ? "destructive" : "success");

export function DashboardPage() {
  const { overview, refresh } = useNetworkStore();
  const pools = useStabilityStore((state) => state.pools);
  const refreshStability = useStabilityStore((state) => state.refresh);
  const [recentLogs, setRecentLogs] = useState<AuditLogEntry[]>([]);

  useEffect(() => {
    void refresh();
    void refreshStability();
    void auditApi.getAuditLogs({ limit: 5 }).then((r) => setRecentLogs(r.logs));
  }, [refresh, refreshStability]);

  return (
    <div className="min-h-full space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total tCeBM Supply</CardDescription>
            <CardTitle>{overview?.totalSupply.toLocaleString() ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Institutions</CardDescription>
            <CardTitle>{overview?.activeInstitutions ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Healthy Pools</CardDescription>
            <CardTitle>{overview?.healthyPools ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Imbalanced Pools</CardDescription>
            <CardTitle>{overview?.imbalancedPools ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Liquidity Health</CardTitle>
            <CardDescription>AMM pair ratios and threshold checks (70/30).</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Pair</TableHead>
                  <TableHead>Ratio</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pools.map((pool) => (
                  <TableRow key={pool.pair}>
                    <TableCell>{pool.pair}</TableCell>
                    <TableCell>{pool.ratioA}/{pool.ratioB}</TableCell>
                    <TableCell>
                      <Badge variant={badgeFromRatio(pool.isImbalanced)}>{pool.isImbalanced ? "IMBALANCED" : "HEALTHY"}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
                {!pools.length ? (
                  <TableRow>
                    <TableCell colSpan={3} className="py-4 text-center text-sm text-muted-foreground">
                      No active liquidity pools.
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Events</CardTitle>
            <CardDescription>Last 5 entries from the compliance audit log.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {recentLogs.map((log) => (
              <div key={log.log_id} className="rounded-md border border-border p-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{log.action}</span>
                  <Badge variant={log.outcome === "SUCCESS" ? "success" : "destructive"}>{log.outcome}</Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  {log.actor}{log.target_subject ? ` → ${log.target_subject}` : ""} · {new Date(log.timestamp).toLocaleString()}
                </p>
              </div>
            ))}
            {!recentLogs.length ? (
              <p className="text-sm text-muted-foreground">No audit events recorded yet.</p>
            ) : null}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
