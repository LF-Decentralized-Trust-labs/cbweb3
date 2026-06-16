import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardFooter,
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
import { isScenarioB } from "../config/scenario";
import { useAccounts, useAuditLogs, useCircuitBreaker, useRegistry } from "../hooks";

type BadgeVariant = "warning" | "success" | "destructive" | "default" | "secondary" | "outline";

function severityVariant(severity: string): BadgeVariant {
  if (severity === "CRITICAL") return "destructive";
  if (severity === "WARNING") return "warning";
  return "secondary";
}

function circuitBreakerVariant(state: string | undefined): BadgeVariant {
  if (!state) return "outline";
  if (state === "LIVE" || state === "ACTIVE") return "success";
  if (state === "HALTED") return "destructive";
  if (state === "RESUMING") return "warning";
  return "outline";
}

export function DashboardPage() {
  const { participants, pendingKyc, fetch: fetchRegistry } = useRegistry();
  const { accounts, fetch: fetchAccounts } = useAccounts();
  const { logs, fetch: fetchAudit } = useAuditLogs();
  const { fetchState, circuitBreaker } = useCircuitBreaker();

  useEffect(() => {
    void fetchRegistry();
    void fetchAccounts();
    void fetchAudit();
    if (isScenarioB) void fetchState();
  }, [fetchRegistry, fetchAccounts, fetchAudit, fetchState]);

  const commercialBanks = participants;
  const activeParticipants = commercialBanks.filter((p) => p.status === "ACTIVE").length;
  const frozenAccounts = accounts.filter((a) => a.frozen).length;
  const pendingKycCount = pendingKyc.length;
  const recentEvents = logs.slice(0, 5);

  return (
    <div className="space-y-4">
      {isScenarioB ? (
        <Card className={circuitBreaker?.state === "HALTED" ? "border-destructive" : ""}>
          <CardHeader className="pb-2">
            <CardDescription>Circuit Breaker</CardDescription>
            <CardTitle className="flex items-center gap-2">
              <Badge variant={circuitBreakerVariant(circuitBreaker?.state)}>
                {circuitBreaker?.state ?? "UNKNOWN"}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <p className="text-xs text-muted-foreground">
              {circuitBreaker?.state === "HALTED"
                ? "Cross-border swaps are paused network-wide."
                : "Cross-border swaps operational."}
            </p>
          </CardContent>
          <CardFooter className="pt-0">
            <Button asChild variant="ghost" size="sm" className="h-auto p-0 text-xs">
              <Link to="/circuit-breaker">Manage →</Link>
            </Button>
          </CardFooter>
        </Card>
      ) : null}

      <section className="grid gap-4 md:grid-cols-3">
        <Card className={pendingKycCount > 0 ? "border-yellow-500" : ""}>
          <CardHeader className="pb-2">
            <CardDescription>Pending KYC Approvals</CardDescription>
            <CardTitle className="flex items-center gap-2">
              {pendingKycCount}
              {pendingKycCount > 0 && <Badge variant="warning">NEEDS ATTENTION</Badge>}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <p className={`text-xs ${pendingKycCount > 0 ? "text-yellow-600" : "text-muted-foreground"}`}>
              {pendingKycCount > 0 ? "Onboarding requests awaiting review" : "No pending requests"}
            </p>
          </CardContent>
          <CardFooter className="pt-0">
            <Button asChild variant="ghost" size="sm" className="h-auto p-0 text-xs">
              <Link to="/registry">Review KYC →</Link>
            </Button>
          </CardFooter>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Participants</CardDescription>
            <CardTitle>
              {activeParticipants} / {commercialBanks.length}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <p className="text-xs text-muted-foreground">Registered commercial banks</p>
          </CardContent>
          <CardFooter className="pt-0">
            <Button asChild variant="ghost" size="sm" className="h-auto p-0 text-xs">
              <Link to="/accounts">View all →</Link>
            </Button>
          </CardFooter>
        </Card>

        <Card className={frozenAccounts > 0 ? "border-destructive" : ""}>
          <CardHeader className="pb-2">
            <CardDescription>Frozen Accounts</CardDescription>
            <CardTitle className="flex items-center gap-2">
              {frozenAccounts}
              {frozenAccounts > 0 && <Badge variant="destructive">FROZEN</Badge>}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <p className="text-xs text-muted-foreground">
              {frozenAccounts > 0 ? "Accounts suspended by governance" : "No frozen accounts"}
            </p>
          </CardContent>
          <CardFooter className="pt-0">
            <Button asChild variant="ghost" size="sm" className="h-auto p-0 text-xs">
              <Link to="/accounts">Manage →</Link>
            </Button>
          </CardFooter>
        </Card>
      </section>

      <section>
        <Card>
          <CardHeader>
            <CardTitle>Recent Governance Events</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Time</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Outcome</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recentEvents.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-center text-muted-foreground">
                      No governance events recorded yet.
                    </TableCell>
                  </TableRow>
                ) : (
                  recentEvents.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className="whitespace-nowrap text-xs">
                        {new Date(log.createdAt).toLocaleTimeString()}
                      </TableCell>
                      <TableCell className="max-w-[200px] truncate text-xs" title={log.action}>
                        {log.action}
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="text-xs">
                          {log.category}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={severityVariant(log.severity)} className="text-xs">
                          {log.severity}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={log.outcome === "SUCCESS" ? "default" : "destructive"} className="text-xs">
                          {log.outcome}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
          <CardFooter>
            <Button asChild variant="ghost" size="sm" className="h-auto p-0 text-xs">
              <Link to="/audit">View all audit logs →</Link>
            </Button>
          </CardFooter>
        </Card>
      </section>
    </div>
  );
}
