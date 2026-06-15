import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@cbweb3/ui";
import { formatCeBM } from "../../types";

type BalanceWidgetProps = {
  balance: string | null;
  decimals: number | null;
  symbol?: string | null;
  loading?: boolean;
};

export function BalanceWidget({
  balance,
  decimals,
  symbol,
  loading = false,
}: BalanceWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>tCeBM Balance</CardDescription>
        <CardTitle>
          {loading ? "Loading..." : formatCeBM(balance ?? "0", decimals ?? 18, symbol)}
        </CardTitle>
      </CardHeader>
    </Card>
  );
}
