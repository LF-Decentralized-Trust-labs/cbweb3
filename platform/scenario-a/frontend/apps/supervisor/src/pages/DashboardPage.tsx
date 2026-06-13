import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect } from "react";
import { useNetworkStore, useStabilityStore, useWebsocketStore } from "../stores";

const badgeFromState = (state: string): "default" | "secondary" | "destructive" | "warning" => {
  if (state === "HTLC_STATE_SETTLED") return "default";
  if (state === "HTLC_STATE_LOCKED" || state === "HTLC_STATE_PENDING") return "secondary";
  if (state === "HTLC_STATE_REFUNDED" || state === "HTLC_STATE_INVALID") return "destructive";
  return "warning"; // SETTLING, REFUNDING
};

export function DashboardPage() {
  const { overview, refresh } = useNetworkStore();
  const { htlcs, refresh: refreshStability } = useStabilityStore();
  const events = useWebsocketStore((state) => state.events);

  useEffect(() => {
    void refresh();
    void refreshStability();
  }, [refresh, refreshStability]);

  return (
    <div className="min-h-full space-y-4">
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Institutions</CardDescription>
            <CardTitle>{overview?.activeInstitutions ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active HTLCs</CardDescription>
            <CardTitle>{overview?.activeHTLCs ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Settlements</CardDescription>
            <CardTitle>{overview?.pendingSettlements ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>HTLC Settlement Status</CardTitle>
            <CardDescription>Active and recent cross-border HTLC contracts.</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>Sender</TableHead>
                  <TableHead>Receiver</TableHead>
                  <TableHead>State</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {htlcs.map((h) => (
                  <TableRow key={h.id}>
                    <TableCell className="font-mono text-xs">{h.id.slice(0, 10)}…</TableCell>
                    <TableCell className="font-mono text-xs">{h.sender}</TableCell>
                    <TableCell className="font-mono text-xs">{h.receiver}</TableCell>
                    <TableCell>
                      <Badge variant={badgeFromState(h.state)}>{h.state}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
                {!htlcs.length ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-sm text-muted-foreground">No active HTLCs</TableCell>
                  </TableRow>
                ) : null}
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
