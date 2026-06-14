import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useMemo } from "react";
import { Link } from "react-router-dom";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { StatCard } from "../components/dashboard/StatCard";
import { poolSideInfo, sideRoleLabel } from "../features/liquidity/format";
import { useLiquidityStore } from "../features/liquidity/liquidity.store";
import { useAccounts, useAuditLogs, useCircuitBreaker, useRegistry } from "../hooks";
import { useAuthStore, usePaymentStore, useTransferLimitsStore } from "../stores";
import type { ParticipantStatus } from "../types/registry.types";
import {
  PaymentStatus,
  currencyFromTokenSymbol,
  formatTokenAmount,
  normalizePaymentStatus,
} from "../types";

const POOL_PAIR = (import.meta.env.VITE_POOL_PAIR ?? "W-BRL-ARS").trim() || "W-BRL-ARS";

type BadgeVariant = "warning" | "success" | "destructive" | "default" | "secondary" | "outline";

function lifecycleVariant(status: string): BadgeVariant {
  const s = status.toUpperCase();
  if (["ACTIVE", "LIVE", "APPROVED", "SETTLED", "SUCCESS"].some((k) => s.includes(k))) return "success";
  if (["HALTED", "REJECTED", "REVOKED", "FAILED", "FROZEN", "CRITICAL", "RECONCILIATION"].some((k) => s.includes(k))) {
    return "destructive";
  }
  if (["PENDING", "REQUESTED", "RESUME", "WARNING", "EMPTY"].some((k) => s.includes(k))) return "warning";
  return "outline";
}

function countByStatus(records: { status: PaymentStatus | string | number }[]) {
  let Pending = 0;
  let Approved = 0;
  let Rejected = 0;
  for (const record of records) {
    const normalized = normalizePaymentStatus(record.status);
    if (normalized === PaymentStatus.PENDING) Pending += 1;
    else if (normalized === PaymentStatus.APPROVED) Approved += 1;
    else if (normalized === PaymentStatus.REJECTED || normalized === PaymentStatus.MINT_FAILED) Rejected += 1;
  }
  return { Pending, Approved, Rejected };
}

function pendingCount(records: { status: PaymentStatus | string | number }[]): number {
  return records.filter((r) => normalizePaymentStatus(r.status) === PaymentStatus.PENDING).length;
}

const PARTICIPANT_STATUSES: ParticipantStatus[] = [
  "ACTIVE",
  "KYC_APPROVED",
  "CREDENTIAL_REQUESTED",
  "PENDING",
  "FROZEN",
  "REVOKED",
];

const chartConfig = {
  count: { label: "Count", color: "hsl(var(--chart-1))" },
  Pending: { label: "Pending", color: "hsl(var(--chart-3))" },
  Approved: { label: "Approved", color: "hsl(var(--chart-1))" },
  Rejected: { label: "Rejected", color: "hsl(var(--chart-4))" },
};

const QUICK_ACTIONS = [
  { to: "/circuit-breaker", label: "Circuit Breaker", primary: true },
  { to: "/registry", label: "Registry & KYC" },
  { to: "/liquidity", label: "Liquidity" },
  { to: "/swap-monitor", label: "Swap Monitor" },
  { to: "/oversight", label: "Oversight" },
];

export function DashboardPage() {
  const { fetch: fetchRegistry, participants, pendingKyc } = useRegistry();
  const { fetch: fetchAccounts, accounts } = useAccounts();
  const { fetch: fetchAudit, logs } = useAuditLogs();
  const { fetchState, circuitBreaker } = useCircuitBreaker();

  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);
  const tokenSymbol = usePaymentStore((state) => state.tokenSymbol);
  const tokenDecimals = usePaymentStore((state) => state.tokenDecimals);

  const poolStatus = useLiquidityStore((state) => state.poolStatus);
  const fetchPoolStatus = useLiquidityStore((state) => state.fetchPoolStatus);

  const limits = useTransferLimitsStore((state) => state.limits);
  const fetchLimits = useTransferLimitsStore((state) => state.fetch);

  const profile = useAuthStore((state) => state.profile);

  useEffect(() => {
    void fetchRegistry();
    void fetchAccounts();
    void fetchAudit();
    void fetchState();
    void fetchPayments();
    void fetchPoolStatus(POOL_PAIR);
    void fetchLimits();
  }, [fetchRegistry, fetchAccounts, fetchAudit, fetchState, fetchPayments, fetchPoolStatus, fetchLimits]);

  const tDecimals = tokenDecimals ?? 18;
  const nationalCurrency = currencyFromTokenSymbol(tokenSymbol);

  const pendingDeposits = pendingCount(deposits);
  const pendingPledges = pendingCount(escrows);
  const pendingRedeems = pendingCount(redeems);
  const pendingKycCount = pendingKyc.length;

  const activeParticipants = participants.filter((p) => p.status === "ACTIVE").length;
  const frozenAccounts = accounts.filter((a) => a.frozen).length;
  const counterpart = poolStatus?.counterpart_commit ?? null;
  const poolLabel = poolStatus?.pool_status ?? "UNKNOWN";

  const participantsData = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const p of participants) counts[p.status] = (counts[p.status] ?? 0) + 1;
    return PARTICIPANT_STATUSES.filter((s) => counts[s]).map((s) => ({
      status: s.replace("CREDENTIAL_", "CRED_"),
      count: counts[s] ?? 0,
    }));
  }, [participants]);

  const breakdownData = [
    { type: "Deposits", ...countByStatus(deposits) },
    { type: "Pledges", ...countByStatus(escrows) },
    { type: "Redeems", ...countByStatus(redeems) },
  ];

  const trendData = useMemo(() => {
    const now = new Date();
    const counts: Record<string, number> = {};
    for (const record of [...deposits, ...escrows, ...redeems]) {
      const key = record.created_at?.slice(0, 10);
      if (key) counts[key] = (counts[key] ?? 0) + 1;
    }
    const days: { day: string; count: number }[] = [];
    for (let i = 6; i >= 0; i -= 1) {
      const d = new Date(now);
      d.setDate(now.getDate() - i);
      const key = d.toISOString().slice(0, 10);
      days.push({ day: d.toLocaleDateString(undefined, { month: "short", day: "numeric" }), count: counts[key] ?? 0 });
    }
    return days;
  }, [deposits, escrows, redeems]);

  return (
    <div className="space-y-4">
      {/* Identity */}
      <Card>
        <CardHeader>
          <CardTitle>Central Bank · {profile?.bankId ?? profile?.subject ?? "Governance"}</CardTitle>
          <CardDescription>
            {profile?.country ? `${profile.country} · ` : ""}
            {profile?.roles?.length ? profile.roles.join(", ") : "Supervisory console"}
          </CardDescription>
        </CardHeader>
      </Card>

      {/* Quick actions */}
      <div className="flex flex-wrap gap-2">
        {QUICK_ACTIONS.map((action) => (
          <Button key={action.to} asChild size="sm" variant={action.primary ? "default" : "outline"}>
            <Link to={action.to}>{action.label}</Link>
          </Button>
        ))}
      </div>

      {/* Monetary controls / status */}
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Circuit Breaker</CardDescription>
            <CardTitle className="text-lg">
              <Badge variant={lifecycleVariant(circuitBreaker?.state ?? "UNKNOWN")}>
                {circuitBreaker?.state ?? "UNKNOWN"}
              </Badge>
            </CardTitle>
            <p className="text-xs text-muted-foreground">
              {circuitBreaker?.state === "HALTED" ? "Swaps are paused network-wide." : "Swaps operational."}
            </p>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Hub Pool ({POOL_PAIR})</CardDescription>
            <CardTitle className="text-lg">
              <Badge variant={lifecycleVariant(poolLabel)}>{poolLabel}</Badge>
              {poolStatus?.imbalance_flag ? (
                <Badge variant="warning" className="ml-2">
                  Imbalanced
                </Badge>
              ) : null}
            </CardTitle>
            <p className="text-xs text-muted-foreground">
              {poolStatus?.current_ratio ? `Current ratio ${poolStatus.current_ratio}` : "No reserves reported."}
            </p>
          </CardHeader>
        </Card>
        <StatCard
          label="Active Participants"
          value={activeParticipants}
          hint={`${participants.length} registered total`}
        />
      </section>

      {/* Approval / gatekeeping queues */}
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="Pending Deposits"
          value={pendingDeposits}
          footer={
            <Button asChild size="sm" variant="outline">
              <Link to="/deposits-approval">Review</Link>
            </Button>
          }
        />
        <StatCard
          label="Pending Pledges"
          value={pendingPledges}
          footer={
            <Button asChild size="sm" variant="outline">
              <Link to="/escrows-approval">Review</Link>
            </Button>
          }
        />
        <StatCard
          label="Pending Redeems"
          value={pendingRedeems}
          footer={
            <Button asChild size="sm" variant="outline">
              <Link to="/redeems-approval">Review</Link>
            </Button>
          }
        />
        <StatCard
          label="Pending KYC"
          value={pendingKycCount}
          hint="Credential requests awaiting approval"
          footer={
            <Button asChild size="sm" variant="outline">
              <Link to="/registry">Review</Link>
            </Button>
          }
        />
      </section>

      {/* Liquidity coordination + supervision */}
      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Liquidity Coordination</CardTitle>
            <CardDescription>Counterpart commits awaiting your matching deposit</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {counterpart ? (
              <div className="space-y-2 rounded border border-amber-300 bg-amber-50 p-3 text-sm">
                <p>
                  A counterpart committed{" "}
                  <strong>{formatTokenAmount(counterpart.amount, tDecimals)}</strong> to the{" "}
                  <strong>
                    {sideRoleLabel(poolSideInfo(counterpart.side, POOL_PAIR, nationalCurrency), {
                      fallbackLabel: `side ${counterpart.side}`,
                    })}
                  </strong>{" "}
                  side.
                </p>
                {counterpart.suggested_match_amount ? (
                  <p>
                    Suggested match on your side:{" "}
                    <strong>{formatTokenAmount(counterpart.suggested_match_amount, tDecimals)}</strong>
                  </p>
                ) : null}
                <Button asChild size="sm">
                  <Link to="/liquidity">Match &amp; Activate</Link>
                </Button>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No pending counterpart commits for {POOL_PAIR}.</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Supervision</CardTitle>
            <CardDescription>Account freezes and transfer-limit policies</CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-2 gap-2">
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Frozen Accounts</p>
              <p className="text-lg font-semibold">{frozenAccounts}</p>
            </div>
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Transfer Limits</p>
              <p className="text-lg font-semibold">{limits.length}</p>
            </div>
          </CardContent>
        </Card>
      </section>

      {/* Audit feed */}
      <Card>
        <CardHeader>
          <CardTitle>Recent Governance Events</CardTitle>
          <CardDescription>Latest entries from the compliance audit log</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>When</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>Severity</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.slice(0, 8).map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="text-muted-foreground">
                    {new Date(log.createdAt).toLocaleString()}
                  </TableCell>
                  <TableCell>{log.action}</TableCell>
                  <TableCell>{log.category}</TableCell>
                  <TableCell>
                    <Badge variant={lifecycleVariant(log.severity)}>{log.severity}</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {logs.length === 0 ? <p className="pt-2 text-sm text-muted-foreground">No audit events.</p> : null}
        </CardContent>
      </Card>

      {/* Charts */}
      <section className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Participants by Status</CardTitle>
            <CardDescription>Network registry composition</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={participantsData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="status" tickLine={false} axisLine={false} interval={0} fontSize={10} />
                <YAxis allowDecimals={false} width={28} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="count" fill="var(--color-count)" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Approvals by Status</CardTitle>
            <CardDescription>Deposits, pledges and redeems</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={breakdownData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="type" tickLine={false} axisLine={false} />
                <YAxis allowDecimals={false} width={28} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <ChartLegend content={<ChartLegendContent />} />
                <Bar dataKey="Pending" stackId="s" fill="var(--color-Pending)" />
                <Bar dataKey="Approved" stackId="s" fill="var(--color-Approved)" />
                <Bar dataKey="Rejected" stackId="s" fill="var(--color-Rejected)" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Approvals (7 days)</CardTitle>
            <CardDescription>Daily request volume</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={trendData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="day" tickLine={false} axisLine={false} />
                <YAxis allowDecimals={false} width={28} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="count" fill="var(--color-count)" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
