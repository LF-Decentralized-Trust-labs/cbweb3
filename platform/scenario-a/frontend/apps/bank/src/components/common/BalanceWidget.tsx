// SPDX-License-Identifier: Apache-2.0

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@cbweb3/ui";
import { formatCeBM } from "../../types";

type BalanceWidgetProps = {
  balance: string | null;
  loading?: boolean;
};

export function BalanceWidget({
  balance,
  loading = false,
}: BalanceWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>tCeBM Balance</CardDescription>
        <CardTitle>
          {loading ? "Loading..." : formatCeBM(balance ?? "0")}
        </CardTitle>
      </CardHeader>
    </Card>
  );
}
