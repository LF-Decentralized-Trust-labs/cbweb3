// SPDX-License-Identifier: Apache-2.0

import { Badge, Card, CardContent, CardDescription, CardHeader, CardTitle, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect } from "react";
import { CopyableValue } from "../components/common/CopyableValue";
import { PartyCell } from "../components/common/PartyCell";
import { useNetworkStore, useStabilityStore } from "../stores";

const badgeFromState = (state: string): "default" | "secondary" | "destructive" | "warning" => {
  if (state === "HTLC_STATE_SETTLED") return "default";
  if (state === "HTLC_STATE_LOCKED" || state === "HTLC_STATE_PENDING") return "secondary";
  if (state === "HTLC_STATE_REFUNDED" || state === "HTLC_STATE_INVALID") return "destructive";
  return "warning"; // SETTLING, REFUNDING
};

export function DashboardPage() {
  const { overview, refresh } = useNetworkStore();
  const { htlcs, refresh: refreshStability } = useStabilityStore();

  useEffect(() => {
    void refresh();
    void refreshStability();
  }, [refresh, refreshStability]);

  return (
    <div className="min-h-full space-y-4">
      <section className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Institutions</CardDescription>
            <CardTitle>{overview?.activeInstitutions ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active HTLCs</CardDescription>
            <CardTitle>{overview?.activeHTLCs ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Settlements</CardDescription>
            <CardTitle>{overview?.pendingSettlements ?? "-"}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>HTLC Settlement Status</CardTitle>
          <CardDescription>Active and recent cross-border HTLC contracts.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Contract ID</TableHead>
                <TableHead>Sender</TableHead>
                <TableHead>Receiver</TableHead>
                <TableHead>Hash Lock</TableHead>
                <TableHead>Zeto Ref</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>State</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {htlcs.map((h) => (
                <TableRow key={h.id}>
                  <TableCell><CopyableValue value={h.id} truncate={14} /></TableCell>
                  <TableCell><PartyCell address={h.sender} name={h.senderName} /></TableCell>
                  <TableCell><PartyCell address={h.receiver} name={h.receiverName} /></TableCell>
                  <TableCell><CopyableValue value={h.hashLock} truncate={14} /></TableCell>
                  <TableCell><CopyableValue value={h.zetoLockRef} truncate={14} /></TableCell>
                  <TableCell className="text-xs">{h.expiresAt ? new Date(h.expiresAt).toLocaleString() : "—"}</TableCell>
                  <TableCell>
                    <Badge variant={badgeFromState(h.state)}>{h.state}</Badge>
                  </TableCell>
                </TableRow>
              ))}
              {!htlcs.length ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-center text-sm text-muted-foreground">No active HTLCs</TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
