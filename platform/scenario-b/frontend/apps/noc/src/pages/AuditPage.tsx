// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useAuditLogs } from "../hooks";
import { useAlertStore } from "../stores";

const actionLabel: Record<string, string> = {
  ACKNOWLEDGE_ALERT: "Acknowledge",
  DISMISS_ALERT: "Dismiss",
};

export function AuditPage() {
  const { logs, fetch } = useAuditLogs();
  const { resolvedAlerts, fetchResolvedAlerts } = useAlertStore();
  const [search, setSearch] = useState("");

  useEffect(() => {
    void fetch();
    void fetchResolvedAlerts();
  }, [fetch, fetchResolvedAlerts]);

  const filtered = useMemo(() => {
    if (!search.trim()) return logs;
    const term = search.toLowerCase();
    return logs.filter(
      (record) =>
        record.actor.toLowerCase().includes(term) ||
        record.action.toLowerCase().includes(term) ||
        record.target_id.toLowerCase().includes(term) ||
        record.detail.toLowerCase().includes(term),
    );
  }, [logs, search]);

  const filteredResolved = useMemo(() => {
    if (!search.trim()) return resolvedAlerts;
    const term = search.toLowerCase();
    return resolvedAlerts.filter(
      (a) =>
        a.title.toLowerCase().includes(term) ||
        a.severity.toLowerCase().includes(term) ||
        a.root_cause_sig.toLowerCase().includes(term),
    );
  }, [resolvedAlerts, search]);

  return (
    <div className="space-y-4">
      <Input
        placeholder="Search actor, action, target, detail…"
        value={search}
        onChange={(event) => setSearch(event.target.value)}
      />

      <Card>
        <CardHeader>
          <CardTitle>Audit Trail</CardTitle>
          <CardDescription>Immutable log of operator actions.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Target</TableHead>
                <TableHead>Detail</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-muted-foreground">No audit records</TableCell>
                </TableRow>
              )}
              {filtered.map((record) => (
                <TableRow key={record.id}>
                  <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                    {new Date(record.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="font-medium">{record.actor}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {actionLabel[record.action] ?? record.action}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{record.target_id}</TableCell>
                  <TableCell className="text-sm">{record.detail}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Resolved Alerts</CardTitle>
          <CardDescription>Alerts that have been resolved and are no longer active.</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Resolved At</TableHead>
                <TableHead>Title</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Root Cause Sig</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredResolved.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-muted-foreground">No resolved alerts</TableCell>
                </TableRow>
              )}
              {filteredResolved.map((alert) => (
                <TableRow key={alert.id}>
                  <TableCell className="text-xs text-muted-foreground">
                    {alert.resolved_at ? new Date(alert.resolved_at).toLocaleString() : "—"}
                  </TableCell>
                  <TableCell className="font-medium">{alert.title}</TableCell>
                  <TableCell>
                    <Badge variant={alert.severity === "CRITICAL" || alert.severity === "HIGH" ? "destructive" : alert.severity === "WARNING" ? "warning" : "secondary"}>
                      {alert.severity}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground truncate max-w-xs">
                    {alert.root_cause_sig}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
