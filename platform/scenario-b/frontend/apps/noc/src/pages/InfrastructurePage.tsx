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
import { useEffect } from "react";
import { useInfrastructure } from "../hooks";

const statusVariant: Record<string, "default" | "secondary" | "warning" | "destructive"> = {
  HEALTHY: "default",
  DEGRADED: "warning",
  DOWN: "destructive",
};

export function InfrastructurePage() {
  const { nodes, status, fetch } = useInfrastructure();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle>Infrastructure Health</CardTitle>
          <CardDescription>Monitor service and node health across runtime topology.</CardDescription>
        </div>
        <Button onClick={() => void fetch()} disabled={status === "loading"}>
          Refresh
        </Button>
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
            {nodes.map((node) => (
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
