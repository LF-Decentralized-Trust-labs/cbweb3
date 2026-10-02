// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  // ChartContainer,
  // ChartLegend,
  // ChartLegendContent,
  // ChartTooltip,
  // ChartTooltipContent,
} from "@cbweb3/ui";
import { useEffect } from "react";
import { Link } from "react-router-dom";
// import {
//   Bar,
//   BarChart,
//   CartesianGrid,
//   Cell,
//   Pie,
//   PieChart,
//   XAxis,
//   YAxis,
// } from "recharts";
import { BalanceWidget } from "../components/common/BalanceWidget";
import {
  useHtlcStore,
  usePaymentStore,
  useStatementStore,
  useTokenStore,
} from "../stores";
import { type Movement, PaymentStatus, formatCeBM, formatFiatUnits, normalizePaymentStatus } from "../types";

const kindLabel: Record<string, string> = {
  deposit: "Deposit",
  tokenisation: "Reserve Tokenisation",
  redeem: "Redeem",
  pvp_settlement: "PvP Settlement",
};

// Amounts are base units; the scale comes from the store because the token decides it,
// not this component (ADR-009). Formatted per token, with a directional sign.
function formatMovementAmount(
  movement: Movement,
  scales: { tCeBMDecimals: number; fiatDecimals: number; fiatSymbol: string },
): string {
  const formatted =
    movement.token === "tCeBM"
      ? formatCeBM(movement.amount, scales.tCeBMDecimals)
      : formatFiatUnits(movement.amount, scales.fiatDecimals, scales.fiatSymbol);
  return `${movement.direction === "credit" ? "+" : "-"} ${formatted}`;
}

// const statusVariant = (status: string): "warning" | "default" | "success" | "destructive" | "outline" => {
//   const variants: Record<string, "warning" | "default" | "success" | "destructive"> = {
//     PENDING: "warning",
//     LOCKED: "default",
//     SETTLED: "success",
//     FAILED: "destructive",
//     CONFIRMED: "success",
//   };
//   return variants[status] ?? "outline";
// };

export function DashboardPage() {
  const fetchToken = useTokenStore((state) => state.fetch);
  // const tokenBalance = useTokenStore((state) => state.balance);
  // const tokenTransactions = useTokenStore((state) => state.transactions);

  const fetchHtlc = useHtlcStore((state) => state.fetchAll);
  const htlcLocks = useHtlcStore((state) => state.locks);

  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const paymentBalance = usePaymentStore((state) => state.balance);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const fiatDecimals = usePaymentStore((state) => state.fiatDecimals);
  const fiatSymbol = usePaymentStore((state) => state.fiatSymbol);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const paymentStatus = usePaymentStore((state) => state.status);
  const deposits = usePaymentStore((state) => state.deposits);
  const escrows = usePaymentStore((state) => state.escrows);
  const redeems = usePaymentStore((state) => state.redeems);

  const fetchStatement = useStatementStore((state) => state.fetchAll);
  const movements = useStatementStore((state) => state.movements);

  useEffect(() => {
    void fetchToken();
    void fetchHtlc();
    void fetchPayments();
    void fetchStatement();
  }, [fetchToken, fetchHtlc, fetchPayments, fetchStatement]);

  // const liquidityData = [
  //   { name: "Public", value: Number(tokenBalance?.publicBalance ?? 0) },
  //   { name: "Private", value: Number(tokenBalance?.privateBalance ?? 0) },
  // ];

  // const txData = tokenTransactions.slice(0, 7).map((tx, index) => ({
  //   name: `${tx.kind}-${index + 1}`,
  //   amount: Number(tx.amount),
  // }));

  // const chartConfig = {
  //   value: { label: "Amount", color: "hsl(var(--primary))" },
  //   Public: { label: "Public", color: "hsl(var(--primary))" },
  //   Private: { label: "Private", color: "hsl(var(--chart-2))" },
  // };

  const lockedCount = htlcLocks.filter(
    (lock) => lock.state === "HTLC_STATE_LOCKED",
  ).length;
  const processingCount = htlcLocks.filter(
    (lock) => lock.state === "HTLC_STATE_SETTLING" || lock.state === "HTLC_STATE_REFUNDING",
  ).length;
  const settledCount = htlcLocks.filter(
    (lock) => lock.state === "HTLC_STATE_SETTLED",
  ).length;
  const refundedCount = htlcLocks.filter(
    (lock) => lock.state === "HTLC_STATE_REFUNDED",
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

  const htlcBadgeVariant = (
    state: string,
  ): "warning" | "default" | "success" | "destructive" | "outline" => {
    if (state === "HTLC_STATE_LOCKED") return "warning";
    if (state === "HTLC_STATE_SETTLED") return "success";
    if (state === "HTLC_STATE_REFUNDED") return "destructive";
    if (state === "HTLC_STATE_SETTLING" || state === "HTLC_STATE_REFUNDING") return "default";
    return "outline";
  };

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <BalanceWidget
          balance={paymentBalance}
          decimals={tCeBMDecimals}
          loading={paymentStatus === "loading" && paymentBalance === null}
        />
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fiat Reserve Balance</CardDescription>
            <CardTitle>
              {paymentStatus === "loading" && fiatBalance === null
                ? "Loading..."
                : formatFiatUnits(fiatBalance ?? "0", fiatDecimals, fiatSymbol)}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Deposits</CardDescription>
            <CardTitle>{pendingDeposits}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Pledges</CardDescription>
            <CardTitle>{pendingPledges}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingRedeems}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      {/* <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Public Liquidity</CardDescription>
            <CardTitle>{tokenBalance?.publicBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="secondary">Domestic</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Private Liquidity</CardDescription>
            <CardTitle>{tokenBalance?.privateBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">Shielded</Badge>
          </CardContent>
        </Card>
      </section>

      <section className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Liquidity Overview</CardTitle>
          <CardDescription>Domestic and private tCeBM balances</CardDescription>
        </CardHeader>
        <CardContent>
        <p className="text-sm">Public: {tokenBalance?.publicBalance ?? "-"} tCeBM</p>
        <p className="text-sm">Private: {tokenBalance?.privateBalance ?? "-"} tCeBM</p>
        </CardContent>
      </Card>
      </section> */}

      {/* <section className="grid gap-4 lg:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Liquidity Mix</CardTitle>
            <CardDescription>Public vs Private balances</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <PieChart>
                <Pie
                  data={liquidityData}
                  dataKey="value"
                  nameKey="name"
                  outerRadius={80}
                >
                  {liquidityData.map((entry) => (
                    <Cell
                      key={entry.name}
                      fill={
                        entry.name === "Public"
                          ? "var(--color-Public)"
                          : "var(--color-Private)"
                      }
                    />
                  ))}
                </Pie>
                <ChartTooltip content={<ChartTooltipContent />} />
                <ChartLegend content={<ChartLegendContent />} />
              </PieChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Recent Amounts</CardTitle>
            <CardDescription>Last transaction values</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={txData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="name" hide />
                <YAxis />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar
                  dataKey="amount"
                  fill="var(--color-value)"
                  radius={[6, 6, 0, 0]}
                />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>
      </section> */}

      {/* <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle>Recent Transactions</CardTitle>
          <CardDescription>Latest token lifecycle events</CardDescription>
        </CardHeader>
        <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tokenTransactions.slice(0, 5).map((tx) => (
              <TableRow key={tx.id}>
                <TableCell>{tx.kind}</TableCell>
                <TableCell>{tx.amount}</TableCell>
                <TableCell>
                  <Badge variant={statusVariant(tx.status)}>{tx.status}</Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {!tokenTransactions.length ? <p className="pt-2 text-sm text-muted-foreground">No transactions yet.</p> : null}
        </CardContent>
      </Card> */}

      <Card className="md:col-span-2">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Recent Movements</CardTitle>
            <CardDescription>Latest tokenized-fiat and tCeBM movements</CardDescription>
          </div>
          <Button asChild size="sm" variant="outline">
            <Link to="/statement">View statement</Link>
          </Button>
        </CardHeader>
        <CardContent>
          <div className="space-y-2">
            {movements.slice(0, 5).map((movement) => (
              <div
                key={movement.id}
                className="flex items-center justify-between rounded border border-border px-3 py-2 text-sm"
              >
                <span className="text-muted-foreground">
                  {new Date(movement.timestamp).toLocaleString()} · {kindLabel[movement.kind] ?? movement.kind} · {movement.token}
                </span>
                <span className="flex items-center gap-2">
                  <span className="font-medium">{formatMovementAmount(movement, { tCeBMDecimals, fiatDecimals, fiatSymbol })}</span>
                  <Badge variant={movement.direction === "credit" ? "success" : "destructive"}>
                    {movement.direction === "credit" ? "Credit" : "Debit"}
                  </Badge>
                </span>
              </div>
            ))}
            {!movements.length ? (
              <p className="text-sm text-muted-foreground">No movements yet.</p>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card className="md:col-span-2">
        <CardHeader>
          <CardTitle>Cross-border Locks</CardTitle>
          <CardDescription>Scenario A status</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3 grid gap-2 md:grid-cols-4">
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Locked</p>
              <p className="text-lg font-semibold">{lockedCount}</p>
            </div>
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Processing</p>
              <p className="text-lg font-semibold">{processingCount}</p>
            </div>
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Settled</p>
              <p className="text-lg font-semibold">{settledCount}</p>
            </div>
            <div className="rounded border border-border p-3">
              <p className="text-xs text-muted-foreground">Refunded</p>
              <p className="text-lg font-semibold">{refundedCount}</p>
            </div>
          </div>
          <div className="mb-3 flex flex-wrap gap-2">
            <Button asChild size="sm">
              <Link to="/htlc/new">Initiate PvP Transfer</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to="/htlc">View PvP Transfers</Link>
            </Button>
          </div>
          <div className="space-y-2">
            {htlcLocks.slice(0, 5).map((lock) => (
              <div
                key={lock.contract_id}
                className="flex items-center justify-between rounded border border-border px-3 py-2 text-sm"
              >
                <span>
                  {lock.contract_id} · {lock.receiver}
                </span>
                <Badge variant={htlcBadgeVariant(lock.state)}>
                  {lock.state.replace("HTLC_STATE_", "")}
                </Badge>
              </div>
            ))}
            {!htlcLocks.length ? (
              <p className="text-sm text-muted-foreground">No active locks.</p>
            ) : null}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
