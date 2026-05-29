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

const healthVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  HEALTHY: "default",
  DEGRADED: "warning",
  OFFLINE: "destructive",
  UNKNOWN: "secondary",
};

export function InfrastructurePage() {
  const { spokes, selectedSpokeId, fetchSpokes, selectSpoke } = useSpokeStore();
  const { components, status, fetchComponents, clearComponents } = useInfrastructureStore();
  const { fallbackPollingSeconds } = useUiStore();
  const navigate = useNavigate();

  useEffect(() => {
    void fetchSpokes();
  }, [fetchSpokes]);

  // Immediately re-fetch (and clear stale data) when the selected spoke changes
  useEffect(() => {
    if (selectedSpokeId) {
      clearComponents();
      void fetchComponents(selectedSpokeId);
    }
  }, [selectedSpokeId, fetchComponents, clearComponents]);

  usePolling(() => {
    if (selectedSpokeId) {
      void fetchComponents(selectedSpokeId);
    }
  }, fallbackPollingSeconds);

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 gap-4">
        <div>
          <CardTitle>Infrastructure Health</CardTitle>
          <CardDescription>Monitor component health per spoke.</CardDescription>
        </div>
        <div className="flex items-center gap-2">
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
          <Button
            variant="outline"
            onClick={() => selectedSpokeId && void fetchComponents(selectedSpokeId)}
            disabled={status === "loading"}
          >
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {components.filter((c) => c.type !== "CACTI_RELAY").length === 0 && status !== "loading" ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            {selectedSpokeId ? "No components found for this spoke." : "Select a spoke to view components."}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Endpoint</TableHead>
                <TableHead>Block #</TableHead>
                <TableHead>Last Check</TableHead>
                <TableHead>Status</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {components.filter((c) => c.type !== "CACTI_RELAY").map((c) => (
                <TableRow key={c.id}>
                  <TableCell className="font-medium">{c.name}</TableCell>
                  <TableCell>{c.type}</TableCell>
                  <TableCell className="max-w-xs truncate text-xs text-muted-foreground">{c.endpoint || "—"}</TableCell>
                  <TableCell>{c.last_block_number ?? "—"}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {c.last_checked_at ? new Date(c.last_checked_at).toLocaleTimeString() : "—"}
                  </TableCell>
                  <TableCell>
                    <Badge variant={healthVariant[c.health_status] ?? "secondary"}>{c.health_status}</Badge>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
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
  );
}

