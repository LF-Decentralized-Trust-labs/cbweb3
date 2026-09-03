// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect } from "react";
import { useReconciliation } from "../hooks";
import { formatAmountString } from "../lib/format";


export function ReconciliationPage() {
  const { tvl, delta, fetch, status, error } = useReconciliation();

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Spoke ↔ Hub Delta</CardTitle>
          <CardDescription>Regional bridge reconciliation status and threshold severity.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>Spoke supply: {delta ? formatAmountString(delta.spokeSupply) : "-"}</p>
          <p>Hub mirror: {delta ? formatAmountString(delta.hubMirror) : "-"}</p>
          <p>Delta: {delta ? formatAmountString(delta.delta) : "-"}</p>
          <Badge variant={delta?.severity === "CRITICAL" ? "destructive" : "secondary"}>{delta?.severity ?? "-"}</Badge>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>TVL by Region</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Region</TableHead>
                <TableHead>TVL</TableHead>
                <TableHead>Updated At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tvl.map((snapshot) => (
                <TableRow key={snapshot.region}>
                  <TableCell>{snapshot.region}</TableCell>
                  <TableCell>{formatAmountString(snapshot.tvl)}</TableCell>
                  <TableCell>{new Date(snapshot.updatedAt).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {status === "loading" ? <p className="mt-2 text-sm text-muted-foreground">Refreshing reconciliation metrics...</p> : null}
          {error ? <p className="mt-2 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
