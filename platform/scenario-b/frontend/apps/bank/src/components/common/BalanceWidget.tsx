// SPDX-License-Identifier: Apache-2.0

import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@cbweb3/ui";
import { formatCeBM, formatTokenAmount, tokenUnitPrefix } from "../../types";

type BalanceWidgetProps = {
  balance: string | null;
  decimals: number | null;
  symbol?: string | null;
  loading?: boolean;
  // When true, render just the numeric value (the card description already names the
  // token, so the trailing unit label is redundant).
  hideSymbol?: boolean;
};

export function BalanceWidget({
  balance,
  decimals,
  symbol,
  loading = false,
  hideSymbol = false,
}: BalanceWidgetProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{tokenUnitPrefix(symbol)} Balance</CardDescription>
        <CardTitle>
          {loading
            ? "Loading..."
            : hideSymbol
              ? formatTokenAmount(balance ?? "0", decimals ?? 18)
              : formatCeBM(balance ?? "0", decimals ?? 18, symbol)}
        </CardTitle>
      </CardHeader>
    </Card>
  );
}
