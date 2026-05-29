import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { useAlerts, useInfrastructure, usePoolStability, useRelays, useTelemetry } from "../hooks";

export function DashboardPage() {
  const { nodes, fetch: fetchInfrastructure } = useInfrastructure();
  const { relays, fetch: fetchRelays } = useRelays();
  const { pools, fetch: fetchPools } = usePoolStability();
  const { alerts } = useAlerts();
  const { frames, fetchSnapshot } = useTelemetry();

  useEffect(() => {
    void fetchInfrastructure();
    void fetchRelays();
    void fetchPools();
    void fetchSnapshot();
  }, [fetchInfrastructure, fetchRelays, fetchPools, fetchSnapshot]);

  const critical = alerts.filter((alert) => alert.severity === "CRITICAL").length;
  const degradedNodes = nodes.filter((node) => node.status !== "HEALTHY").length;
  const worstRelay = relays.length ? Math.max(...relays.map((relay) => relay.latencyP95Ms)) : 0;
  const breachedPools = pools.filter((pool) => pool.breached7030).length;

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Critical Alerts</CardDescription>
            <CardTitle>{critical}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Degraded Nodes</CardDescription>
            <CardTitle>{degradedNodes}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Worst Relay p95</CardDescription>
            <CardTitle>{worstRelay} ms</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pool Breaches (70/30)</CardDescription>
            <CardTitle>{breachedPools}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Recent Telemetry Frames</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Component</TableHead>
                  <TableHead>CPU</TableHead>
                  <TableHead>Mem</TableHead>
                  <TableHead>Latency</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {frames.slice(0, 6).map((frame) => (
                  <TableRow key={frame.id}>
                    <TableCell>{frame.component}</TableCell>
                    <TableCell>{frame.cpuPct}%</TableCell>
                    <TableCell>{frame.memoryPct}%</TableCell>
                    <TableCell>{frame.latencyMs} ms</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Active Alert Feed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {alerts.slice(0, 8).map((alert) => (
              <div key={alert.id} className="rounded-md border border-border p-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">{alert.source}</span>
                  <Badge variant={alert.severity === "CRITICAL" ? "destructive" : alert.severity === "WARNING" ? "warning" : "secondary"}>
                    {alert.severity}
                  </Badge>
                </div>
                <p className="text-muted-foreground">{alert.message}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
