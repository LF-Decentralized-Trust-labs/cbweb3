// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { paymentApi } from "../services/api";
import { useAuthStore, usePaymentStore } from "../stores";
import type { BalanceResponse } from "../types";
import {
  formatCeBM,
  formatFiatUnits,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
  PaymentStatus,
  tCeBMUnitLabel,
} from "../types";

const institutionName = (import.meta.env.VITE_INSTITUTION_NAME ?? "").trim();

const truncateAddress = (address?: string | null) =>
  address && address.length > 12 ? `${address.slice(0, 6)}…${address.slice(-4)}` : (address ?? "—");

const chartConfig = {
  count: { label: "Transactions", color: "hsl(var(--chart-1))" },
};

type Activity = {
  id: string;
  kind: "Issuance" | "Tokenisation" | "Redemption";
  requester: string;
  amount: string;
  status: unknown;
  createdAt: string;
};

export function DashboardPage() {
  const { deposits, escrows, redeems, tokenSymbol, tokenDecimals, fetchAll } = usePaymentStore();
  const user = useAuthStore((state) => state.user);

  const [balance, setBalance] = useState<BalanceResponse | null>(null);

  useEffect(() => {
    void fetchAll();
    paymentApi.getBalance().then(setBalance).catch(() => setBalance(null));
  }, [fetchAll]);

  const decimals = balance?.decimals ?? tokenDecimals ?? 18;
  const symbol = balance?.symbol ?? tokenSymbol;

  const countPending = (records: { status: unknown }[]) =>
    records.filter((r) => normalizePaymentStatus(r.status) === PaymentStatus.PENDING).length;

  const recent = useMemo<Activity[]>(() => {
    const combined: Activity[] = [
      ...deposits.map((d) => ({ id: d.id, kind: "Issuance" as const, requester: d.requester_id, amount: formatFiatUnits(d.amount, decimals, symbol), status: d.status, createdAt: d.created_at })),
      ...escrows.map((e) => ({ id: e.id, kind: "Tokenisation" as const, requester: e.requester_id, amount: formatCeBM(e.amount, decimals, symbol), status: e.status, createdAt: e.created_at })),
      ...redeems.map((r) => ({ id: r.id, kind: "Redemption" as const, requester: r.requester_id, amount: formatCeBM(r.amount, decimals, symbol), status: r.status, createdAt: r.created_at })),
    ];
    return combined.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()).slice(0, 8);
  }, [deposits, escrows, redeems, decimals, symbol]);

  // Daily transaction counts for the last 7 days, from this spoke's own operations
  // (issuances + tokenisations + redemptions), keyed by their created_at date.
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
      {/* Welcome / identity */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle>Welcome, {institutionName || user?.institutionId || user?.name || "Treasury"}</CardTitle>
            <CardDescription>
              Treasury operations · Wallet {truncateAddress(user?.walletAddress)}
              {user?.role ? ` · ${user.role}` : ""}
            </CardDescription>
          </div>
          {user?.authorizedIssuer ? <Badge variant="success">Authorised issuer</Badge> : null}
        </CardHeader>
      </Card>

      {/* KPI row */}
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{tCeBMUnitLabel(symbol)} balance</CardDescription>
            <CardTitle>{balance ? formatCeBM(balance.balance, decimals, symbol) : "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Issuance approvals</CardDescription>
            <CardTitle>{countPending(deposits)}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Tokenisation approvals</CardDescription>
            <CardTitle>{countPending(escrows)}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redemption approvals</CardDescription>
            <CardTitle>{countPending(redeems)}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        {/* Transactions per period (spoke operations) */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Transactions (7 days)</CardTitle>
            <CardDescription>Daily issuances, tokenisations and redemptions in your spoke</CardDescription>
          </CardHeader>
          <CardContent className="h-64">
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

        {/* Recent operations */}
        <Card>
          <CardHeader>
            <CardTitle>Recent Operations</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Type</TableHead>
                  <TableHead>Requester</TableHead>
                  <TableHead>Amount</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {recent.map((op) => (
                  <TableRow key={`${op.kind}-${op.id}`}>
                    <TableCell>{op.kind}</TableCell>
                    <TableCell>{op.requester}</TableCell>
                    <TableCell>{op.amount}</TableCell>
                    <TableCell>
                      <Badge variant={getPaymentStatusVariant(op.status)}>{getPaymentStatusLabel(op.status)}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
                {recent.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-muted-foreground">
                      No operations yet.
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
