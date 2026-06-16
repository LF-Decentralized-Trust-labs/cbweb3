import {
  Badge,
  Button,
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
import { useEffect, useState } from "react";
import { useInfrastructure } from "../hooks";

const statusVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  HEALTHY: "default",
  DEGRADED: "warning",
  DOWN: "destructive",
};

const ALL_NETWORKS = "All Networks";

export function InfrastructurePage() {
  const { nodes, status, fetch } = useInfrastructure();
  const [selectedNetwork, setSelectedNetwork] = useState<string>(ALL_NETWORKS);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const networks = [ALL_NETWORKS, ...Array.from(new Set(nodes.map((n) => n.network))).sort()];
  const filtered = selectedNetwork === ALL_NETWORKS ? nodes : nodes.filter((n) => n.network === selectedNetwork);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle>Infrastructure Health</CardTitle>
          <CardDescription>Monitor service and node health across the Hub and all spoke networks.</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <select
            className="rounded-md border border-border bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
            value={selectedNetwork}
            onChange={(e) => setSelectedNetwork(e.target.value)}
          >
            {networks.map((net) => (
              <option key={net} value={net}>
                {net}
              </option>
            ))}
          </select>
          <Button onClick={() => void fetch()} disabled={status === "loading"}>
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Node</TableHead>
              <TableHead>Component</TableHead>
              <TableHead>Network</TableHead>
              <TableHead>Region</TableHead>
              <TableHead>Uptime</TableHead>
              <TableHead>Sync Lag</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((node) => (
              <TableRow key={node.id}>
                <TableCell className="font-medium">{node.id}</TableCell>
                <TableCell>{node.component}</TableCell>
                <TableCell>{node.network}</TableCell>
                <TableCell>{node.region}</TableCell>
                <TableCell>{node.uptimePct}%</TableCell>
                <TableCell>{node.syncLagBlocks} blocks</TableCell>
                <TableCell>
                  <Badge variant={statusVariant[node.status] ?? "secondary"}>{node.status}</Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
