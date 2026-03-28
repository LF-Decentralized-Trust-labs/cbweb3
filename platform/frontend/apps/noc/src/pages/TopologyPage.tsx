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
import { useTopology } from "../hooks";

export function TopologyPage() {
  const { nodes, edges, fetch } = useTopology();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <div className="grid gap-4 xl:grid-cols-3">
      <Card className="xl:col-span-2">
        <CardHeader>
          <CardTitle>Topology Snapshot</CardTitle>
          <CardDescription>Operational graph snapshots from the latest synchronization cycle.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Node</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>Redundant</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {nodes.map((node) => (
                <TableRow key={node.id}>
                  <TableCell className="font-medium">{node.label}</TableCell>
                  <TableCell>{node.kind}</TableCell>
                  <TableCell>{node.redundant ? "YES" : "NO"}</TableCell>
                  <TableCell>
                    <Badge variant={node.status === "HEALTHY" ? "default" : node.status === "DEGRADED" ? "warning" : "destructive"}>
                      {node.status}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Topology Health</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <div className="flex items-center justify-between">
            <span>Total Nodes</span>
            <Badge variant="secondary">{nodes.length}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Total Links</span>
            <Badge variant="secondary">{edges.length}</Badge>
          </div>
          <div className="flex items-center justify-between">
            <span>Healthy Links</span>
            <Badge variant="default">{edges.filter((edge) => edge.healthy).length}</Badge>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
