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
import { useRelays } from "../hooks";

export function RelayStatusPage() {
  const { relays, status, fetch } = useRelays();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle>Relay Health</CardTitle>
          <CardDescription>Track relay latency/error rates and quarantine unstable routes.</CardDescription>
        </div>
        <Button onClick={() => void fetch()} disabled={status === "loading"}>
          Refresh
        </Button>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Relay</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>p50 Latency</TableHead>
              <TableHead>p95 Latency</TableHead>
              <TableHead>Proof Success</TableHead>
              <TableHead>Updated At</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {relays.map((relay) => (
              <TableRow key={relay.id}>
                <TableCell className="font-medium">{relay.route}</TableCell>
                <TableCell>
                  <Badge variant={relay.status === "HEALTHY" ? "default" : relay.status === "DEGRADED" ? "warning" : "destructive"}>
                    {relay.status}
                  </Badge>
                </TableCell>
                <TableCell>{relay.latencyP50Ms} ms</TableCell>
                <TableCell>{relay.latencyP95Ms} ms</TableCell>
                <TableCell>{relay.proofSuccessRatePct}%</TableCell>
                <TableCell>{new Date(relay.updatedAt).toLocaleString()}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
