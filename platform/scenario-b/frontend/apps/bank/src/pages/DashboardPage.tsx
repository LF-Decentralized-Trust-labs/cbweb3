import {
  Badge,
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
  buttonVariants,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Bar, BarChart, CartesianGrid, Cell, XAxis, YAxis } from "recharts";
import { StatCard } from "../components/dashboard/StatCard";
import { useAmmV2Store } from "../features/amm/amm-v2.store";
import { bridgeApi, onboardingApi } from "../services/api";
import { useAuthStore, useFxAgreementStore, usePaymentStore } from "../stores";
import type { BridgedAssetPosition } from "../types/bridge.types";
import {
  PaymentStatus,
  formatCeBM,
  formatFiatUnits,
  formatTokenAmount,
  getPaymentStatusLabel,
  normalizePaymentStatus,
} from "../types";

const POOL_PAIR = (import.meta.env.VITE_POOL_PAIR ?? "W-BRL-ARS").trim() || "W-BRL-ARS";
const STATUS_REFRESH_MS = 30_000;

// rawToNumber converts a base-unit token amount to a JS number for charts/ratios.
// Precision is sufficient for display-scale values (loses precision beyond ~2^53 base units).
function rawToNumber(raw: string | null | undefined, decimals: number): number {
  if (!raw) return 0;
  try {
    return Number(BigInt(raw)) / 10 ** decimals;
  } catch {
    return 0;
  }
}

function truncateAddress(value?: string): string {
  if (!value) return "-";
  return value.length <= 12 ? value : `${value.slice(0, 6)}…${value.slice(-4)}`;
}

type BadgeVariant = "warning" | "success" | "destructive" | "default" | "outline";

function lifecycleVariant(status: string): BadgeVariant {
  const s = status.toUpperCase();
  if (["APPROVED", "SETTLED", "ACTIVE", "RELEASED", "EXECUTED", "CONFIRMED", "LIVE"].some((k) => s.includes(k))) {
    return "success";
  }
  if (["REJECTED", "FAILED", "CANCELLED", "RECONCILIATION", "HALTED"].some((k) => s.includes(k))) {
    return "destructive";
  }
  if (["PENDING", "PROPOSED", "LOCKING", "BURNING", "RESUME"].some((k) => s.includes(k))) {
    return "warning";
  }
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

const chartConfig = {
  value: { label: "Amount", color: "hsl(var(--chart-1))" },
  count: { label: "Requests", color: "hsl(var(--chart-1))" },
  Pending: { label: "Pending", color: "hsl(var(--chart-3))" },
  Approved: { label: "Approved", color: "hsl(var(--chart-1))" },
  Rejected: { label: "Rejected", color: "hsl(var(--chart-4))" },
};

const QUICK_ACTIONS = [
  { to: "/deposits", label: "New Deposit" },
  { to: "/redeems", label: "Redeem" },
  { to: "/transfer", label: "Cross-border Transfer" },
  { to: "/bridge", label: "Bridge to Hub" },
  { to: "/amm", label: "AMM Trade" },
];

export function DashboardPage() {
  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const paymentStatus = usePaymentStore((state) => state.status);
  const tCeBMBalance = usePaymentStore((state) => state.balance);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const fCeBMDecimals = usePaymentStore((state) => state.fCeBMDecimals);
  const tCeBMSymbol = usePaymentStore((state) => state.tCeBMSymbol);
  const fCeBMSymbol = usePaymentStore((state) => state.fCeBMSymbol);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);

  const poolStatus = useAmmV2Store((state) => state.poolStatus);
  const circuitBreakerState = useAmmV2Store((state) => state.circuitBreakerState);
  const fetchPoolStatus = useAmmV2Store((state) => state.fetchPoolStatus);
  const fetchCircuitBreakerState = useAmmV2Store((state) => state.fetchCircuitBreakerState);

  const agreements = useFxAgreementStore((state) => state.agreements);
  const fetchAgreements = useFxAgreementStore((state) => state.fetchAll);

  const profile = useAuthStore((state) => state.profile);

  const [bridgePositions, setBridgePositions] = useState<BridgedAssetPosition[]>([]);
  const [onboardingState, setOnboardingState] = useState<string | null>(null);

  useEffect(() => {
    void fetchPayments();
    void fetchAgreements();

    let active = true;
    const loadStatus = () => {
      void fetchPoolStatus(POOL_PAIR);
      void fetchCircuitBreakerState(POOL_PAIR);
      bridgeApi
        .listPositions()
        .then((positions) => {
          if (active) setBridgePositions(positions);
        })
        .catch(() => {
          /* endpoint optional on older gateways — panel shows empty */
        });
    };
    loadStatus();
    onboardingApi
      .getMyStatus()
      .then((res) => {
        if (active) setOnboardingState(res.status);
      })
      .catch(() => {
        /* compliance status optional */
      });

    const id = window.setInterval(loadStatus, STATUS_REFRESH_MS);
    return () => {
      active = false;
      window.clearInterval(id);
    };
  }, [fetchPayments, fetchAgreements, fetchPoolStatus, fetchCircuitBreakerState]);

  const tDecimals = tCeBMDecimals ?? 18;
  const fDecimals = fCeBMDecimals ?? 18;
  const balanceLoading = paymentStatus === "loading" && tCeBMBalance === null;

  const tCeBMNumber = rawToNumber(tCeBMBalance, tDecimals);
  const fCeBMNumber = rawToNumber(fiatBalance, fDecimals);

  const pendingActions = pendingCount(deposits) + pendingCount(escrows) + pendingCount(redeems);
  const fxProposals = agreements.filter((a) => a.state === "FX_STATE_PROPOSED").length;

  const activeBridge = bridgePositions.filter((p) => p.bridge_state === "ACTIVE");
  const bridgeNeedsAttention = bridgePositions.filter(
    (p) => p.bridge_state === "RECONCILIATION_REQUIRED" || p.relayer_retries > 0,
  ).length;
  const hubValue = activeBridge.reduce((sum, p) => {
    try {
      return sum + BigInt(p.mirrored_amount);
    } catch {
      return sum;
    }
  }, 0n);

  const activity = useMemo(() => {
    const items = [
      ...deposits.map((d) => ({
        id: `dep-${d.id}`,
        type: "Deposit",
        amount: formatFiatUnits(d.amount, fDecimals, fCeBMSymbol),
        status: getPaymentStatusLabel(d.status),
        ts: d.created_at,
      })),
      ...escrows.map((e) => ({
        id: `esc-${e.id}`,
        type: "Tokenisation",
        amount: formatFiatUnits(e.amount, fDecimals, fCeBMSymbol),
        status: getPaymentStatusLabel(e.status),
        ts: e.created_at,
      })),
      ...redeems.map((r) => ({
        id: `red-${r.id}`,
        type: "Redeem",
        amount: formatCeBM(r.amount, tDecimals, tCeBMSymbol),
        status: getPaymentStatusLabel(r.status),
        ts: r.created_at,
      })),
      ...bridgePositions.map((p) => ({
        id: `brg-${p.position_id}`,
        type: "Bridge",
        amount: formatCeBM(p.mirrored_amount, tDecimals, tCeBMSymbol),
        status: p.bridge_state,
        ts: p.updated_at,
      })),
    ];
    return items
      .filter((i) => i.ts)
      .sort((a, b) => Date.parse(b.ts) - Date.parse(a.ts))
      .slice(0, 8);
  }, [deposits, escrows, redeems, bridgePositions, fDecimals, tDecimals, fCeBMSymbol, tCeBMSymbol]);

  const balancesData = [
    { name: "tCeBM", value: tCeBMNumber },
    { name: "fCeBM reserve", value: fCeBMNumber },
  ];

  const breakdownData = [
    { type: "Deposits", ...countByStatus(deposits) },
    { type: "Tokenisations", ...countByStatus(escrows) },
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

  const poolLabel = poolStatus?.pool_status ?? "UNKNOWN";

  return (
    <div className="space-y-4">
      {/* Identity / context */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle>Welcome, {profile?.bankId ?? profile?.subject ?? "Bank"}</CardTitle>
            <CardDescription>
              {profile?.country ? `${profile.country} · ` : ""}Wallet {truncateAddress(profile?.wallet)}
              {profile?.roles?.length ? ` · ${profile.roles.join(", ")}` : ""}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            {onboardingState ? (
              <Badge variant={lifecycleVariant(onboardingState)}>Compliance: {onboardingState}</Badge>
            ) : null}
          </div>
        </CardHeader>
      </Card>

      {/* KPI row */}
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label="tCeBM Balance"
          value={formatCeBM(tCeBMBalance ?? "0", tDecimals, tCeBMSymbol)}
          loading={balanceLoading}
        />
        <StatCard
          label="Fiat Reserve (fCeBM)"
          value={formatFiatUnits(fiatBalance ?? "0", fDecimals, fCeBMSymbol)}
          loading={balanceLoading}
        />
        <StatCard
          label="Pending Actions"
          value={pendingActions}
          hint="Deposits · tokenisations · redeems"
        />
        <StatCard
          label="Bridged to Hub"
          value={formatCeBM(hubValue.toString(), tDecimals, tCeBMSymbol)}
          hint={`${activeBridge.length} active position${activeBridge.length === 1 ? "" : "s"}`}
        />
      </section>

      {/* Status strip */}
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Circuit Breaker</CardDescription>
            <CardTitle className="text-lg">
              <Badge variant={circuitBreakerState ? lifecycleVariant(circuitBreakerState) : "outline"}>
                {circuitBreakerState ?? "UNKNOWN"}
              </Badge>
            </CardTitle>
            <p className="text-xs text-muted-foreground">
              {circuitBreakerState === "HALTED"
                ? "Cross-border swaps are paused."
                : circuitBreakerState === "LIVE"
                  ? "Swaps operational."
                  : "Awaiting status from the hub."}
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
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>FX Proposals Awaiting You</CardDescription>
            <CardTitle className="text-2xl">{fxProposals}</CardTitle>
            <p className="text-xs text-muted-foreground">Proposed agreements pending accept/reject</p>
          </CardHeader>
        </Card>
      </section>

      {/* Quick actions */}
      <div className="flex flex-wrap gap-2">
        {QUICK_ACTIONS.map((action) => (
          <Link key={action.to} to={action.to} className={buttonVariants({ variant: "outline", size: "sm" })}>
            {action.label}
          </Link>
        ))}
      </div>

      {/* Activity + bridge positions */}
      <section className="grid gap-4 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Recent Activity</CardTitle>
            <CardDescription>Latest deposits, tokenisations, redeems and bridge events</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                  <TableHead>Amount</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>When</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {activity.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.type}</TableCell>
                    <TableCell>{item.amount}</TableCell>
                    <TableCell>
                      <Badge variant={lifecycleVariant(item.status)}>{item.status}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(item.ts).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {activity.length === 0 ? (
              <p className="pt-2 text-sm text-muted-foreground">No activity yet.</p>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Bridge Positions</CardTitle>
            <CardDescription>
              Liquidity mirrored at the hub
              {bridgeNeedsAttention > 0 ? ` · ${bridgeNeedsAttention} need attention` : ""}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {bridgePositions.length === 0 ? (
              <p className="text-sm text-muted-foreground">No bridge positions.</p>
            ) : (
              bridgePositions.slice(0, 6).map((p) => (
                <div key={p.position_id} className="flex items-center justify-between gap-2 text-sm">
                  <span className="truncate">
                    {formatTokenAmount(p.mirrored_amount, tDecimals)}{" "}
                    <span className="text-muted-foreground">{p.mirrored_asset}</span>
                  </span>
                  <Badge variant={lifecycleVariant(p.bridge_state)}>{p.bridge_state}</Badge>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </section>

      {/* Charts */}
      <section className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Tokenised vs Reserve</CardTitle>
            <CardDescription>tCeBM in circulation vs fCeBM reserve</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={balancesData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" tickLine={false} axisLine={false} />
                <YAxis hide />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="value" radius={[6, 6, 0, 0]}>
                  {balancesData.map((entry) => (
                    <Cell
                      key={entry.name}
                      fill={entry.name === "tCeBM" ? "hsl(var(--chart-1))" : "hsl(var(--chart-2))"}
                    />
                  ))}
                </Bar>
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Requests by Status</CardTitle>
            <CardDescription>Deposits, tokenisations and redeems</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={breakdownData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="type" tickLine={false} axisLine={false} />
                <YAxis allowDecimals={false} width={28} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <ChartLegend content={<ChartLegendContent />} />
                <Bar dataKey="Pending" stackId="s" fill="var(--color-Pending)" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Approved" stackId="s" fill="var(--color-Approved)" radius={[0, 0, 0, 0]} />
                <Bar dataKey="Rejected" stackId="s" fill="var(--color-Rejected)" radius={[6, 6, 0, 0]} />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Activity (7 days)</CardTitle>
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
