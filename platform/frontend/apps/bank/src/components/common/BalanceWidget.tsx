import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle } from "@cbweb3/ui";
import { formatCeBM } from "../../types";

type BalanceWidgetProps = {
  balance: string | null;
  loading?: boolean;
};

export function BalanceWidget({ balance, loading = false }: BalanceWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>tCeBM Balance</CardDescription>
        <CardTitle>{loading ? "Loading..." : formatCeBM(balance ?? "0")}</CardTitle>
      </CardHeader>
      <CardContent>
        <Badge variant="outline">Available for HTLC, escrow, and redeem flows</Badge>
      </CardContent>
    </Card>
  );
}
