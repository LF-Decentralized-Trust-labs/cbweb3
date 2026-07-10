// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useMemo } from "react";
import { useStatementStore } from "../stores";
import { formatCeBM, formatFiatUnits, type Movement } from "../types";

const shortHash = (value?: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

const kindLabel: Record<string, string> = {
  deposit: "Deposit",
  tokenisation: "Reserve Tokenisation",
  redeem: "Redeem",
};

const tokenLabel: Record<string, string> = {
  fCeBM: "Tokenized Fiat (fCeBM)",
  tCeBM: "tCeBM",
};

// Amounts are integer units; format per token (fiat symbol vs tCeBM) with a
// directional sign.
function formatAmount(movement: Movement): string {
  const formatted = movement.token === "tCeBM" ? formatCeBM(movement.amount) : formatFiatUnits(movement.amount);
  return `${movement.direction === "credit" ? "+" : "-"} ${formatted}`;
}

export function StatementPage() {
  const fetchAll = useStatementStore((state) => state.fetchAll);
  const movements = useStatementStore((state) => state.movements);
  const status = useStatementStore((state) => state.status);
  const error = useStatementStore((state) => state.error);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const { credits, debits } = useMemo(
    () => ({
      credits: movements.filter((m) => m.direction === "credit").length,
      debits: movements.filter((m) => m.direction === "debit").length,
    }),
    [movements],
  );

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">Statement</h1>
        <p className="text-sm text-muted-foreground">
          Consolidated record of tokenized-fiat (fCeBM) and tCeBM movements received and sent by this bank.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Movements</CardDescription>
            <CardTitle>{movements.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Credits (received)</CardDescription>
            <CardTitle>{credits}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Debits (sent)</CardDescription>
            <CardTitle>{debits}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Movements</CardTitle>
            <CardDescription>Most recent first.</CardDescription>
          </div>
          <Button variant="outline" onClick={() => void fetchAll()} disabled={status === "loading"}>
            {status === "loading" ? "Loading..." : "Refresh"}
          </Button>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Token</TableHead>
                <TableHead>Direction</TableHead>
                <TableHead className="text-right">Amount</TableHead>
                <TableHead>Reference</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {movements.map((movement) => (
                <TableRow key={movement.id}>
                  <TableCell>{new Date(movement.timestamp).toLocaleString()}</TableCell>
                  <TableCell>{kindLabel[movement.kind] ?? movement.kind}</TableCell>
                  <TableCell>{tokenLabel[movement.token] ?? movement.token}</TableCell>
                  <TableCell>
                    <Badge variant={movement.direction === "credit" ? "success" : "destructive"}>
                      {movement.direction === "credit" ? "Credit" : "Debit"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right font-medium">{formatAmount(movement)}</TableCell>
                  <TableCell title={movement.reference}>{shortHash(movement.reference)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {error ? <p className="pt-3 text-sm text-destructive">{error}</p> : null}
          {!error && !movements.length ? (
            <p className="pt-3 text-sm text-muted-foreground">
              {status === "loading" ? "Loading movements..." : "No movements found."}
            </p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
