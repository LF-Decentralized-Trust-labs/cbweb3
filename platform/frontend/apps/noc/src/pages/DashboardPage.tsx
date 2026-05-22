import {
  Badge,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { useAlertStore, useInfrastructureStore, useSpokeStore } from "../stores";

const severityVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  INFO: "secondary",
  WARNING: "warning",
  HIGH: "destructive",
  CRITICAL: "destructive",
};

const healthVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  HEALTHY: "default",
  DEGRADED: "warning",
  OFFLINE: "destructive",
  UNKNOWN: "secondary",
};

export function DashboardPage() {
  const { spokes, selectedSpokeId, fetchSpokes } = useSpokeStore();
  const { components, fetchComponents } = useInfrastructureStore();
  const { alerts, fetchAlerts } = useAlertStore();

  useEffect(() => {
    void fetchSpokes();
  }, [fetchSpokes]);

  useEffect(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
      void fetchAlerts(selectedSpokeId);
    } else {
      void fetchAlerts();
    }
  }, [selectedSpokeId, fetchComponents, fetchAlerts]);

  const critical = alerts.filter((a) => a.severity === "CRITICAL" || a.severity === "HIGH").length;
  const degraded = components.filter((c) => c.health_status !== "HEALTHY").length;
  const offline = components.filter((c) => c.health_status === "OFFLINE").length;
  const selectedSpoke = spokes.find((s) => s.id === selectedSpokeId);

  return (
    <div className="space-y-4">
      {selectedSpoke && (
        <p className="text-sm text-muted-foreground">
          Viewing: <span className="font-medium text-foreground">{selectedSpoke.name}</span> ({selectedSpoke.currency_code})
        </p>
      )}

      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Critical / High Alerts</CardDescription>
            <CardTitle className={critical > 0 ? "text-destructive" : ""}>{critical}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Degraded Components</CardDescription>
            <CardTitle className={degraded > 0 ? "text-warning" : ""}>{degraded}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Offline Components</CardDescription>
            <CardTitle className={offline > 0 ? "text-destructive" : ""}>{offline}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Spokes</CardDescription>
            <CardTitle>{spokes.filter((s) => s.active).length}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Component Health</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {components.slice(0, 8).map((c) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{c.type}</TableCell>
                    <TableCell>
                      <Badge variant={healthVariant[c.health_status] ?? "secondary"}>{c.health_status}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
                {components.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-center text-muted-foreground">No components yet</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Active Alert Feed</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {alerts.length === 0 && (
              <p className="py-4 text-center text-sm text-muted-foreground">No active alerts</p>
            )}
            {alerts.slice(0, 8).map((alert) => (
              <div key={alert.id} className="rounded-md border border-border p-2 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium truncate">{alert.title}</span>
                  <Badge variant={severityVariant[alert.severity] ?? "secondary"}>
                    {alert.severity}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">{new Date(alert.created_at).toLocaleString()}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}

