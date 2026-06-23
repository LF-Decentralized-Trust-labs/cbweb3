// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect } from "react";
import { useFundingRequests, useReconciliation, useSupplyAudit } from "../hooks";
import { useWebsocketStore } from "../stores";

const formatAmount = (value: string) => Number(value).toLocaleString();

export function DashboardPage() {
  const { requests, fetchRequests } = useFundingRequests();
  const { supply, operations, fetch: fetchSupply } = useSupplyAudit();
  const { delta, fetch: fetchReconciliation } = useReconciliation();
  const events = useWebsocketStore((state) => state.events);

  useEffect(() => {
    void fetchRequests();
    void fetchSupply();
    void fetchReconciliation();
  }, [fetchRequests, fetchSupply, fetchReconciliation]);

  const pending = requests.filter((request) => request.status === "PENDING").length;

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Circulating Supply</CardDescription>
            <CardTitle>{supply ? formatAmount(supply.circulatingSupply) : "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Funding Requests</CardDescription>
            <CardTitle>{pending}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Last Delta Severity</CardDescription>
            <CardTitle>{delta?.severity ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Recent Operations</CardDescription>
            <CardTitle>{operations.length}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Latest Mint/Burn Operations</CardTitle>
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
                {operations.slice(0, 6).map((operation) => (
                  <TableRow key={operation.id}>
                    <TableCell>{operation.kind}</TableCell>
                    <TableCell>{formatAmount(operation.amount)}</TableCell>
                    <TableCell>
                      <Badge variant={operation.status === "CONFIRMED" ? "default" : "outline"}>{operation.status}</Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

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
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
