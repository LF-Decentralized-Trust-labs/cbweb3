import {
  Card,
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
  useAmmStore,
  usePaymentStore,
  useTokenStore,
} from "../stores";
import { PaymentStatus, normalizePaymentStatus } from "../types";

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

  const refreshPool = useAmmStore((state) => state.refreshPool);
  // const pool = useAmmStore((state) => state.pool);

  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const paymentBalance = usePaymentStore((state) => state.balance);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const paymentStatus = usePaymentStore((state) => state.status);
  const deposits = usePaymentStore((state) => state.deposits);
  const redeems = usePaymentStore((state) => state.redeems);

  useEffect(() => {
    void fetchToken();
    void refreshPool();
    void fetchPayments();
  }, [fetchToken, refreshPool, fetchPayments]);

  // const liquidityData = [
  //   { name: "Public", value: Number(tokenBalance?.publicBalance ?? 0) },
  //   { name: "Private", value: Number(tokenBalance?.privateBalance ?? 0) },
  // ];

  // const txData = tokenTransactions.slice(0, 7).map((tx, index) => ({
  //   name: `${tx.kind}-${index + 1}`,
  //   amount: Number(tx.amount),
  // }));

  // const poolData = [
  //   { name: pool?.tokenA ?? "Token A", value: Number(pool?.reserveA ?? 0) },
  //   { name: pool?.tokenB ?? "Token B", value: Number(pool?.reserveB ?? 0) },
  // ];

  // const chartConfig = {
  //   value: { label: "Amount", color: "hsl(var(--primary))" },
  //   Public: { label: "Public", color: "hsl(var(--primary))" },
  //   Private: { label: "Private", color: "hsl(var(--chart-2))" },
  // };

  const pendingDeposits = deposits.filter(
    (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
  ).length;
  const pendingRedeems = redeems.filter(
    (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
  ).length;

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
            <CardDescription>Pending Deposits</CardDescription>
            <CardTitle>{pendingDeposits}</CardTitle>
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
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pool Health</CardDescription>
            <CardTitle>{pool?.imbalanceFlag ? "Imbalanced" : "Healthy"}</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={statusVariant(pool?.imbalanceFlag ? "FAILED" : "CONFIRMED")}>
              {pool?.imbalanceFlag ? "FAILED" : "CONFIRMED"}
            </Badge>
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

      <Card>
        <CardHeader>
          <CardTitle>AMM Pool Status</CardTitle>
          <CardDescription>Scenario B structural status</CardDescription>
        </CardHeader>
        <CardContent>
        <p className="text-sm">{pool?.tokenA ?? "-"} Reserve: {pool?.reserveA ?? "-"}</p>
        <p className="text-sm">{pool?.tokenB ?? "-"} Reserve: {pool?.reserveB ?? "-"}</p>
        <p className="mt-2">
          <Badge variant={statusVariant(pool?.imbalanceFlag ? "FAILED" : "CONFIRMED")}>
            {pool?.imbalanceFlag ? "FAILED" : "CONFIRMED"}
          </Badge>
        </p>
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

        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">AMM Reserves</CardTitle>
            <CardDescription>Current pool composition</CardDescription>
          </CardHeader>
          <CardContent className="h-56">
            <ChartContainer config={chartConfig} className="h-full w-full">
              <BarChart data={poolData} layout="vertical" margin={{ left: 20 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis type="number" hide />
                <YAxis type="category" dataKey="name" width={90} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar
                  dataKey="value"
                  fill="hsl(var(--chart-2))"
                  radius={[0, 6, 6, 0]}
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

    </div>
  );
}
