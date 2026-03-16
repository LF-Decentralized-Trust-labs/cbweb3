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
import { Link } from "react-router-dom";
import { useAccounts, useAuditLogs, useCircuitBreaker, useRegistry } from "../hooks";

export function DashboardPage() {
  const { participants, fetch: fetchRegistry } = useRegistry();
  const { accounts, fetch: fetchAccounts } = useAccounts();
  const { logs, fetch: fetchAudit } = useAuditLogs();
  const { circuitBreaker, fetchState } = useCircuitBreaker();

  useEffect(() => {
    void fetchRegistry();
    void fetchAccounts();
    void fetchAudit();
    void fetchState();
  }, [fetchRegistry, fetchAccounts, fetchAudit, fetchState]);

  const activeParticipants = participants.filter((item) => item.status === "ACTIVE").length;
  const frozenAccounts = accounts.filter((item) => item.frozen).length;
  const criticalToday = logs.filter((item) => item.severity === "CRITICAL").length;

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Circuit Breaker</CardDescription>
            <CardTitle>
              <Badge variant={circuitBreaker?.state === "HALTED" ? "destructive" : "default"}>
                {circuitBreaker?.state ?? "LIVE"}
              </Badge>
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Participants</CardDescription>
            <CardTitle>{activeParticipants}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Frozen Accounts</CardDescription>
            <CardTitle>{frozenAccounts}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Critical Audit Events</CardDescription>
            <CardTitle>{criticalToday}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Quick Actions</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            <Button asChild>
              <Link to="/circuit-breaker">Go to Circuit Breaker</Link>
            </Button>
            <Button asChild variant="outline">
              <Link to="/registry">View Registry</Link>
            </Button>
            <Button asChild variant="outline">
              <Link to="/accounts">Freeze Controls</Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent Governance Events</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Severity</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.slice(0, 6).map((log) => (
                  <TableRow key={log.id}>
                    <TableCell>{new Date(log.createdAt).toLocaleString()}</TableCell>
                    <TableCell>{log.action}</TableCell>
                    <TableCell>
                      <Badge variant={log.severity === "CRITICAL" ? "destructive" : log.severity === "WARNING" ? "warning" : "secondary"}>
                        {log.severity}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
