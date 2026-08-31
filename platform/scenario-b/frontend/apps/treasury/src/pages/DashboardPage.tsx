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
import { hubReconciliationApi, paymentApi } from "../services/api";
import type { HubReconciliationReport } from "../services/api";
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
  // Hub obligation: what this CB holds for its banks and what part of it its records cannot
  // attribute. Optional — a gateway without Hub access does not serve it, and the card then
  // stays hidden rather than showing a zero nobody computed.
  const [reconciliation, setReconciliation] = useState<HubReconciliationReport | null>(null);

  useEffect(() => {
    hubReconciliationApi
      .get()
      .then(setReconciliation)
      .catch(() => {
        /* not an issuing CB, or no Hub access — the card is simply not shown */
      });
  }, []);

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

      {/* Hub obligation — one number whose expected value is zero. The banks hold no W-token; this
          balance is what this CB owes them while payments are in flight, so anything it cannot
          attribute to a payment is a reportable condition. */}
      {reconciliation ? (
        <Card>
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2 pb-2">
            <div>
              <CardDescription>
                {/* The two non-zero cases are not the same finding: an excess is value the records
                    cannot attribute, a shortfall is value that should be there and is not. */}
                Hub obligation ·{" "}
                {reconciliation.balanced
                  ? "unattributed balance"
                  : reconciliation.unexplained.startsWith("-")
                    ? "SHORTFALL against records"
                    : "unattributed balance"}
              </CardDescription>
              <CardTitle className={reconciliation.balanced ? undefined : "text-destructive"}>
                {reconciliation.balanced ? "Balanced" : formatCeBM(reconciliation.unexplained, decimals, symbol)}
              </CardTitle>
            </div>
            <Badge variant={reconciliation.balanced ? "success" : "destructive"}>
              {reconciliation.balanced ? "Reconciled" : "Needs reconciliation"}
            </Badge>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground">
            <div className="grid gap-1 sm:grid-cols-3">
              <span>On-chain: {formatCeBM(reconciliation.on_chain_balance, decimals, symbol)}</span>
              <span>In flight: {formatCeBM(reconciliation.expected_in_flight, decimals, symbol)}</span>
              {/* Context, not a deduction: a stuck residue is usually still counted inside its
                  parent's in-flight amount, so it is listed rather than netted off. */}
              <span>Already flagged (not deducted): {formatCeBM(reconciliation.stranded_total, decimals, symbol)}</span>
            </div>
            {reconciliation.per_bank.length > 0 ? (
              <div className="mt-2">
                Owed per bank:{" "}
                {reconciliation.per_bank
                  .map((b) => `${b.owner_bank_id} ${formatCeBM(b.amount, decimals, symbol)} (${b.positions})`)
                  .join(" · ")}
              </div>
            ) : null}
            {/* Naming the flagged positions is what makes the figure actionable: OUT is value still
                on this address whose return was given up on, IN is value that never arrived. */}
            {reconciliation.stranded && reconciliation.stranded.length > 0 ? (
              <div className="mt-2">
                Flagged positions:{" "}
                {reconciliation.stranded
                  .map(
                    (s) =>
                      `${s.position_id} ${s.leg}/${s.direction} ${formatCeBM(s.amount, decimals, symbol)} (${s.bridge_state})`,
                  )
                  .join(" · ")}
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

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
