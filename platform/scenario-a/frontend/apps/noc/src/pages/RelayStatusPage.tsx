// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
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
import { ScrollText } from "lucide-react";
import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { usePolling } from "../hooks";
import { useInfrastructureStore, useSpokeStore, useUiStore } from "../stores";

export function RelayStatusPage() {
  const { spokes, selectedSpokeId, fetchSpokes, selectSpoke } = useSpokeStore();
  const { components, fetchComponents } = useInfrastructureStore();
  const { fallbackPollingSeconds } = useUiStore();
  const navigate = useNavigate();

  useEffect(() => {
    void fetchSpokes();
  }, [fetchSpokes]);

  // Fetch as soon as a spoke is known. The polling tick alone is not enough: the spoke
  // list arrives after mount, so the first tick finds no selection and the page shows
  // "no interoperability containers" for a whole polling interval — reading as "this
  // spoke has no relay" on the page an operator opens when a settlement looks stuck.
  useEffect(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
    }
  }, [selectedSpokeId, fetchComponents]);

  usePolling(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
    }
  }, fallbackPollingSeconds);

  const interopComponents = components.filter((c) => c.type === "CACTI_RELAY");

  const healthVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
    HEALTHY: "default",
    DEGRADED: "warning",
    OFFLINE: "destructive",
    UNKNOWN: "secondary",
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between space-y-0 gap-4">
          <div>
            <CardTitle>Interoperability Containers</CardTitle>
            <CardDescription>Cacti relayer containers health per spoke.</CardDescription>
          </div>
          <Select value={selectedSpokeId ?? ""} onValueChange={selectSpoke}>
            <SelectTrigger className="w-48">
              <SelectValue placeholder="Select spoke…" />
            </SelectTrigger>
            <SelectContent>
              {spokes.map((s) => (
                <SelectItem key={s.id} value={s.id}>
                  {s.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </CardHeader>
        <CardContent>
          {!selectedSpokeId ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Select a spoke to view interoperability containers.</p>
          ) : interopComponents.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">No interoperability containers found for this spoke.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Endpoint</TableHead>
                  <TableHead>Last Check</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {interopComponents.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell>{c.type}</TableCell>
                    <TableCell className="max-w-xs truncate text-xs text-muted-foreground">{c.endpoint || "—"}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {c.last_checked_at ? new Date(c.last_checked_at).toLocaleTimeString() : "—"}
                    </TableCell>
                    <TableCell>
                      <Badge variant={healthVariant[c.health_status] ?? "secondary"}>{c.health_status}</Badge>
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        title="View logs"
                        onClick={() => navigate(`/logs/${c.id}?name=${encodeURIComponent(c.name)}`)}
                      >
                        <ScrollText className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
