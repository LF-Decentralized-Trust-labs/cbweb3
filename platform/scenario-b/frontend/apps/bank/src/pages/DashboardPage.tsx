// SPDX-License-Identifier: Apache-2.0

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
import { ammV2Api } from "../services/api/amm-v2.api";
import { bridgeApi, onboardingApi } from "../services/api";
import { useAuthStore, usePaymentStore } from "../stores";
import type { BridgedAssetPosition } from "../types/bridge.types";
import {
  PaymentStatus,
  formatCeBM,
  formatFiatUnits,
  formatTokenAmount,
  getPaymentStatusLabel,
  normalizePaymentStatus,
} from "../types";

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

// prettifyLabel turns kebab/snake identifiers into a human label:
// "bank-itau" → "Bank Itau", "COMMERCIAL_BANK" → "Commercial Bank".
function prettifyLabel(value: string): string {
  return value
    .split(/[-_]+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(" ");
}

// The institution name is baked at build time (per-entity image); /auth/me only
// carries the Keycloak subject (a UUID), so use this for a friendly greeting.
const INSTITUTION_NAME = (import.meta.env.VITE_INSTITUTION_NAME ?? "").trim();

// businessRoles keeps only the RBAC role(s) that matter to the operator, dropping
// Keycloak noise (default-roles-*, offline_access, uma_authorization, lowercase dups).
function businessRoles(roles?: string[]): string[] {
  return (roles ?? [])
    .filter((role) => role.startsWith("ROLE_"))
    .map((role) => prettifyLabel(role.replace(/^ROLE_/, "")));
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
  { to: "/bridge", label: "Bridge to Hub" },
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

  const profile = useAuthStore((state) => state.profile);

  const [bridgePositions, setBridgePositions] = useState<BridgedAssetPosition[]>([]);
  const [onboardingState, setOnboardingState] = useState<string | null>(null);
  const [poolsTotal, setPoolsTotal] = useState<number | null>(null);
  const [poolsActive, setPoolsActive] = useState<number>(0);

  useEffect(() => {
    void fetchPayments();

    let active = true;
    const loadStatus = () => {
      ammV2Api
        .getPairs()
        .then((pairs) => {
          if (!active) return;
          setPoolsTotal(pairs.length);
          setPoolsActive(pairs.filter((p) => p.status === "ACTIVE").length);
        })
        .catch(() => {
          /* pools endpoint optional — card shows a dash */
        });
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
  }, [fetchPayments]);

  const tDecimals = tCeBMDecimals ?? 18;
  const fDecimals = fCeBMDecimals ?? 18;
  const balanceLoading = paymentStatus === "loading" && tCeBMBalance === null;

  const tCeBMNumber = rawToNumber(tCeBMBalance, tDecimals);
  const fCeBMNumber = rawToNumber(fiatBalance, fDecimals);

  const pendingActions = pendingCount(deposits) + pendingCount(escrows) + pendingCount(redeems);

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

  return (
    <div className="space-y-4">
      {/* Identity / context */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle>
              Welcome, {INSTITUTION_NAME ? prettifyLabel(INSTITUTION_NAME) : (profile?.bankId ?? profile?.subject ?? "Bank")}
            </CardTitle>
            <CardDescription>
              {(() => {
                const roles = businessRoles(profile?.roles);
                const parts = [
                  profile?.country || null,
                  profile?.wallet ? `Wallet ${truncateAddress(profile.wallet)}` : null,
                  roles.length ? roles.join(", ") : null,
                ].filter(Boolean);
                return parts.length ? parts.join(" · ") : "Commercial bank operator";
              })()}
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
          value={formatTokenAmount(tCeBMBalance ?? "0", tDecimals)}
          loading={balanceLoading}
        />
        <StatCard
          label="Fiat Reserve (fCeBM)"
          value={formatTokenAmount(fiatBalance ?? "0", fDecimals)}
          loading={balanceLoading}
        />
        <StatCard
          label="Pending Actions"
          value={pendingActions}
          hint="Deposits · tokenisations · redeems"
        />
        <Link to="/pools" className="rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring">
          <StatCard
            label="Available Pools"
            value={poolsActive}
            hint={poolsTotal === null ? "Loading…" : `${poolsTotal} total · view details`}
            loading={poolsTotal === null}
          />
        </Link>
      </section>

      {/* Quick actions */}
      <div className="flex flex-wrap gap-2">
        {QUICK_ACTIONS.map((action) => (
          <Link key={action.to} to={action.to} className={buttonVariants({ variant: "outline", size: "sm" })}>
            {action.label}
          </Link>
        ))}
      </div>

      {/* Recent activity */}
      <section>
        <Card>
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
