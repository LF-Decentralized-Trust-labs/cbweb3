import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { Link } from "react-router-dom";
import {
  useAccounts,
  useAuditLogs,
  useCircuitBreaker,
  useRegistry,
} from "../hooks";
import { useHtlcMonitorStore, usePaymentStore } from "../stores";
import { PaymentStatus, normalizePaymentStatus } from "../types";

export function DashboardPage() {
  const { fetch: fetchRegistry } = useRegistry();
  const { fetch: fetchAccounts } = useAccounts();
  const { fetch: fetchAudit } = useAuditLogs();
  const { fetchState } = useCircuitBreaker();
  const htlcLocks = useHtlcMonitorStore((state) => state.locks);
  const fetchHtlc = useHtlcMonitorStore((state) => state.fetch);
  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);

  useEffect(() => {
    void fetchRegistry();
    void fetchAccounts();
    void fetchAudit();
    void fetchState();
    void fetchHtlc();
    void fetchPayments();
  }, [
    fetchRegistry,
    fetchAccounts,
    fetchAudit,
    fetchState,
    fetchHtlc,
    fetchPayments,
  ]);

  // const activeParticipants = participants.filter((item) => item.status === "ACTIVE").length;
  // const frozenAccounts = accounts.filter((item) => item.frozen).length;
  // const criticalToday = logs.filter((item) => item.severity === "CRITICAL").length;
  const lockedCount = htlcLocks.filter(
    (item) => item.state === "HTLC_STATE_LOCKED",
  ).length;
  const settledCount = htlcLocks.filter(
    (item) => item.state === "HTLC_STATE_SETTLED",
  ).length;
  const refundedCount = htlcLocks.filter(
    (item) => item.state === "HTLC_STATE_REFUNDED",
  ).length;
  const pendingDeposits = deposits.filter(
    (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
  ).length;
  const pendingPledges = escrows.filter(
    (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
  ).length;
  const pendingRedeems = redeems.filter(
    (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
  ).length;

  return (
    <div className="space-y-4">
      {/* <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
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
      </section> */}

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
            <Button asChild variant="outline">
              <Link to="/htlc-monitor">PvP Settlement</Link>
            </Button>
          </CardContent>
        </Card>

        {/* <Card>
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
                    <TableCell>
                      {new Date(log.createdAt).toLocaleString()}
                    </TableCell>
                    <TableCell>{log.action}</TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          log.severity === "CRITICAL"
                            ? "destructive"
                            : log.severity === "WARNING"
                              ? "warning"
                              : "secondary"
                        }
                      >
                        {log.severity}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card> */}
      </section>

      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Deposits</CardDescription>
            <CardTitle>{pendingDeposits}</CardTitle>
          </CardHeader>
          <CardContent>
            <Button asChild size="sm">
              <Link to="/deposits-approval">Review Deposits</Link>
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Pledges</CardDescription>
            <CardTitle>{pendingPledges}</CardTitle>
          </CardHeader>
          <CardContent>
            <Button asChild size="sm">
              <Link to="/escrows-approval">Review Pledges</Link>
            </Button>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingRedeems}</CardTitle>
          </CardHeader>
          <CardContent>
            <Button asChild size="sm">
              <Link to="/redeems-approval">Review Redeems</Link>
            </Button>
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>HTLC Cross-Spoke Activity</CardTitle>
          <CardDescription>
            Operational monitor for inter-spoke atomic settlement contracts.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-2 md:grid-cols-3">
          <div className="rounded border border-border p-3">
            <p className="text-xs text-muted-foreground">Active Locks</p>
            <p className="text-lg font-semibold">{lockedCount}</p>
          </div>
          <div className="rounded border border-border p-3">
            <p className="text-xs text-muted-foreground">Settled</p>
            <p className="text-lg font-semibold">{settledCount}</p>
          </div>
          <div className="rounded border border-border p-3">
            <p className="text-xs text-muted-foreground">Refunded</p>
            <p className="text-lg font-semibold">{refundedCount}</p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
