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
  /** Base-unit scale of `balance`. Required: without it the widget cannot tell
   *  1000 wei from 1000 whole tokens (ADR-009). */
  decimals: number;
  loading?: boolean;
};

export function BalanceWidget({
  balance,
  decimals,
  loading = false,
}: BalanceWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>tCeBM Balance</CardDescription>
        <CardTitle>
          {loading ? "Loading..." : formatCeBM(balance ?? "0", decimals)}
        </CardTitle>
      </CardHeader>
    </Card>
  );
}
