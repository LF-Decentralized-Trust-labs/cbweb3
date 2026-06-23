// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { circuitBreakerApi, liquidityApi, paymentApi } from "../services/api";
import { useAuthStore, usePaymentStore, useWebsocketStore } from "../stores";
import type { BalanceResponse, LpBalanceResponse, PoolStatus } from "../types";
import { CB_STATE, type CircuitBreakerStatus } from "../types/circuit-breaker.types";
import { formatCeBM, formatFiatUnits, formatTokenAmount, getPaymentStatusLabel, getPaymentStatusVariant, normalizePaymentStatus, PaymentStatus, tCeBMUnitLabel } from "../types";

const configuredPoolPair = (import.meta.env.VITE_POOL_PAIR ?? "W-BRL-ARS").trim() || "W-BRL-ARS";
const institutionName = (import.meta.env.VITE_INSTITUTION_NAME ?? "").trim();

const truncateAddress = (address?: string | null) => (address && address.length > 12 ? `${address.slice(0, 6)}…${address.slice(-4)}` : (address ?? "—"));

const fmtRatio = (raw?: string) => {
  if (!raw) return "—";
  const n = Number(raw);
  return Number.isFinite(n) ? n.toFixed(4) : raw;
};

type Activity = {
  id: string;
  kind: "Issuance" | "Tokenisation" | "Redemption";
  requester: string;
  amount: string;
  status: unknown;
  createdAt: string;
};

const cbBadge = (state?: CircuitBreakerStatus["state"]): { variant: "success" | "warning" | "destructive" | "outline"; label: string } => {
  switch (state) {
    case CB_STATE.HALTED:
      return { variant: "destructive", label: "HALTED" };
    case CB_STATE.RESUME_PENDING:
      return { variant: "warning", label: "RESUME PENDING" };
    case CB_STATE.LIVE:
      return { variant: "success", label: "LIVE" };
    default:
      return { variant: "outline", label: "UNKNOWN" };
  }
};

export function DashboardPage() {
  const { deposits, escrows, redeems, tokenSymbol, tokenDecimals, fetchAll } = usePaymentStore();
  const events = useWebsocketStore((state) => state.events);
  const user = useAuthStore((state) => state.user);

  const [balance, setBalance] = useState<BalanceResponse | null>(null);
  const [pool, setPool] = useState<PoolStatus | null>(null);
  const [lp, setLp] = useState<LpBalanceResponse | null>(null);
  const [cb, setCb] = useState<CircuitBreakerStatus | null>(null);

  useEffect(() => {
    void fetchAll();
    paymentApi.getBalance().then(setBalance).catch(() => setBalance(null));
    liquidityApi.getPoolStatus(configuredPoolPair).then(setPool).catch(() => setPool(null));
    liquidityApi.getLpBalance().then(setLp).catch(() => setLp(null));
    circuitBreakerApi.getStatus(configuredPoolPair).then(setCb).catch(() => setCb(null));
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

  const breaker = cbBadge(cb?.state);

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

      {/* Circuit breaker banner */}
      <Card className={cb?.state === CB_STATE.HALTED ? "border-destructive" : undefined}>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2 pb-3">
          <div>
            <CardDescription>Hub circuit breaker · {configuredPoolPair}</CardDescription>
            <CardTitle className="text-base">
              {cb?.state === CB_STATE.HALTED
                ? "Swaps are halted"
                : cb?.state === CB_STATE.RESUME_PENDING
                  ? "Resume request pending signatures"
                  : cb?.state === CB_STATE.LIVE
                    ? "Trading is live"
                    : "Circuit-breaker state unavailable"}
            </CardTitle>
            {cb?.pause_reason ? <CardDescription>Reason: {cb.pause_reason}</CardDescription> : null}
          </div>
          <Badge variant={breaker.variant}>{breaker.label}</Badge>
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
        {/* Hub pool & FX + CB liquidity position */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-2">
            <CardTitle>Hub Pool &amp; FX — {pool?.pool_pair ?? configuredPoolPair}</CardTitle>
            {pool?.imbalance_flag ? <Badge variant="warning">Imbalanced</Badge> : pool ? <Badge variant="success">Balanced</Badge> : null}
          </CardHeader>
          <CardContent>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
              <div>
                <dt className="text-muted-foreground">FX rate (ratio)</dt>
                <dd className="font-medium">{fmtRatio(pool?.current_ratio)}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Reserve A</dt>
                <dd className="font-medium">{pool ? formatTokenAmount(pool.reserve_a, decimals) : "—"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Reserve B</dt>
                <dd className="font-medium">{pool ? formatTokenAmount(pool.reserve_b, decimals) : "—"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">LP providers</dt>
                <dd className="font-medium">{pool?.total_lp_count ?? "—"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Your pool share</dt>
                <dd className="font-medium">{lp ? `${lp.share_percentage.toFixed(2)}%` : "—"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Your LP shares</dt>
                <dd className="font-medium">{lp ? formatTokenAmount(lp.lp_shares, decimals) : "—"}</dd>
              </div>
              <div>
                <dt className="text-muted-foreground">Pending commits</dt>
                <dd className="font-medium">{pool?.pending_commits?.length ?? 0}</dd>
              </div>
            </dl>
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

      {/* Real-time event stream */}
      <Card>
        <CardHeader>
          <CardTitle>Real-time Event Stream</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {events.slice(0, 8).map((event) => (
            <div key={event.id} className="rounded-md border border-border p-2 text-sm">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{event.type}</span>
                <Badge variant={event.severity === "CRITICAL" ? "destructive" : "secondary"}>{event.severity}</Badge>
              </div>
              <p className="text-muted-foreground">{event.message}</p>
            </div>
          ))}
          {events.length === 0 ? <p className="text-sm text-muted-foreground">No live events.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
