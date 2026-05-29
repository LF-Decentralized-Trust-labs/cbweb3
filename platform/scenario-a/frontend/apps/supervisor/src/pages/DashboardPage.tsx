import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect } from "react";
import { useNetworkStore, useStabilityStore, useWebsocketStore } from "../stores";

const badgeFromRatio = (imbalanced: boolean): "destructive" | "success" => (imbalanced ? "destructive" : "success");

export function DashboardPage() {
  const { overview, refresh } = useNetworkStore();
  const pools = useStabilityStore((state) => state.pools);
  const refreshStability = useStabilityStore((state) => state.refresh);
  const events = useWebsocketStore((state) => state.events);

  useEffect(() => {
    void refresh();
    void refreshStability();
  }, [refresh, refreshStability]);

  return (
    <div className="min-h-full space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
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
            <CardDescription>Cross-border Agreements</CardDescription>
            <CardTitle>{overview?.activeAgreements ?? "-"}</CardTitle>
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
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Real-time Alerts (SSE)</CardTitle>
            <CardDescription>Latest backend telemetry notifications.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {events.slice(0, 8).map((event) => (
              <div key={event.id} className="rounded-md border border-border p-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{event.type}</span>
                  <Badge variant={event.severity === "CRITICAL" ? "destructive" : "warning"}>{event.severity}</Badge>
                </div>
                <p className="text-muted-foreground">{event.message}</p>
              </div>
            ))}
            {!events.length ? <p className="text-sm text-muted-foreground">Waiting for realtime events...</p> : null}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
