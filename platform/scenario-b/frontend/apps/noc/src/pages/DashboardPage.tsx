// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { XCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { AlertDetailModal } from "../components/alerts/AlertDetailModal";
import { usePolling } from "../hooks";
import { useAlertStore, useInfrastructureStore, usePoolStore, useSpokeStore, useUiStore } from "../stores";
import { filterActiveAlerts } from "../stores/ui.settings";

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
  const { spokes, selectedSpokeId, fetchSpokes, selectSpoke } = useSpokeStore();
  const { components, fetchComponents, clearComponents } = useInfrastructureStore();
  const { alerts, fetchAlerts, dismissAlert, clearSelectedAlert } = useAlertStore();
  const { pools, fetch: fetchPools } = usePoolStore();
  const { fallbackPollingSeconds, muteAlerts } = useUiStore();
  const [openAlertId, setOpenAlertId] = useState<string | null>(null);

  useEffect(() => {
    void fetchSpokes();
    void fetchPools();
  }, [fetchSpokes, fetchPools]);

  // Component health is per spoke; on "All Spokes" the alert feed goes platform-wide and
  // the component tables are cleared instead of showing the previous spoke's rows.
  useEffect(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
      void fetchAlerts(selectedSpokeId);
    } else {
      clearComponents();
      void fetchAlerts();
    }
  }, [selectedSpokeId, fetchComponents, clearComponents, fetchAlerts]);

  usePolling(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
      void fetchAlerts(selectedSpokeId);
    } else {
      void fetchAlerts();
    }
  }, fallbackPollingSeconds);

  const critical = alerts.filter((a) => a.state === "ACTIVE" && (a.severity === "CRITICAL" || a.severity === "HIGH")).length;
  const degraded = components.filter((c) => c.health_status !== "HEALTHY").length;
  const offline = components.filter((c) => c.health_status === "OFFLINE").length;
  const breachedPools = pools.filter((p) => p.breached7030).length;

  const activeAlerts = filterActiveAlerts(alerts, muteAlerts);

  function handleSelectSpoke(value: string) {
    selectSpoke(value === "all" ? null : value);
  }

  function handleCloseModal() {
    setOpenAlertId(null);
    clearSelectedAlert();
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <span className="text-sm text-muted-foreground shrink-0">Viewing spoke:</span>
        <Select value={selectedSpokeId ?? "all"} onValueChange={handleSelectSpoke}>
          <SelectTrigger className="w-56">
            <SelectValue placeholder="All Spokes" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Spokes</SelectItem>
            {spokes.filter((s) => s.active).map((s) => (
              <SelectItem key={s.id} value={s.id}>
                {s.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

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
            <CardTitle className={degraded > 0 ? "text-warning" : ""}>
              {selectedSpokeId ? degraded : "—"}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Offline Components</CardDescription>
            <CardTitle className={offline > 0 ? "text-destructive" : ""}>
              {selectedSpokeId ? offline : "—"}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pool Breaches (70/30)</CardDescription>
            <CardTitle className={breachedPools > 0 ? "text-destructive" : ""}>{breachedPools}</CardTitle>
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
                    <TableCell colSpan={3} className="text-center text-muted-foreground">
                      {selectedSpokeId ? "No components yet" : "Select a spoke to see component health"}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <CardTitle>Active Alert Feed</CardTitle>
              {muteAlerts && <Badge variant="secondary">CRITICAL ONLY</Badge>}
            </div>
          </CardHeader>
          <CardContent className="space-y-2">
            {activeAlerts.length === 0 && (
              <p className="py-4 text-center text-sm text-muted-foreground">No active alerts</p>
            )}
            {activeAlerts.slice(0, 8).map((alert) => (
              <div
                key={alert.id}
                className="rounded-md border border-border p-2 text-sm cursor-pointer hover:bg-accent transition-colors"
                onClick={() => setOpenAlertId(alert.id)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => e.key === "Enter" && setOpenAlertId(alert.id)}
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium truncate flex-1">{alert.title}</span>
                  <Badge variant={severityVariant[alert.severity] ?? "secondary"}>
                    {alert.severity}
                  </Badge>
                  {alert.acknowledged_by && (
                    <Badge variant="secondary" className="shrink-0">ACK</Badge>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 w-6 p-0 shrink-0 text-muted-foreground hover:text-destructive"
                    title="Dismiss alert"
                    onClick={(e) => {
                      e.stopPropagation();
                      void dismissAlert(alert.id);
                    }}
                  >
                    <XCircle className="h-4 w-4" />
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">{new Date(alert.created_at).toLocaleString()}</p>
              </div>
            ))}
          </CardContent>
        </Card>
      </section>

      <AlertDetailModal alertId={openAlertId} onClose={handleCloseModal} />
    </div>
  );
}
