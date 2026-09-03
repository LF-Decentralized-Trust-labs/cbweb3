// SPDX-License-Identifier: Apache-2.0

import { Card, CardContent, CardHeader, CardTitle, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useAuditLogs } from "../hooks";
import type { AuditLogEntry } from "../types";

export function AuditPage() {
  const { logs, fetch, status, error } = useAuditLogs();
  const [category, setCategory] = useState<"ALL" | AuditLogEntry["category"]>("ALL");
  const [severity, setSeverity] = useState<"ALL" | AuditLogEntry["severity"]>("ALL");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const filtered = useMemo(
    () =>
      logs.filter(
        (item) => (category === "ALL" || item.category === category) && (severity === "ALL" || item.severity === severity),
      ),
    [logs, category, severity],
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>Audit Logs</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 md:grid-cols-2">
          <Select value={category} onValueChange={(value) => setCategory(value as typeof category)}>
            <SelectTrigger>
              <SelectValue placeholder="Filter by category" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All categories</SelectItem>
              <SelectItem value="AUTH">AUTH</SelectItem>
              <SelectItem value="FUNDING">FUNDING</SelectItem>
              <SelectItem value="TREASURY">TREASURY</SelectItem>
              <SelectItem value="KYC">KYC</SelectItem>
              <SelectItem value="RECONCILIATION">RECONCILIATION</SelectItem>
            </SelectContent>
          </Select>
          <Select value={severity} onValueChange={(value) => setSeverity(value as typeof severity)}>
            <SelectTrigger>
              <SelectValue placeholder="Filter by severity" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All severities</SelectItem>
              <SelectItem value="INFO">INFO</SelectItem>
              <SelectItem value="WARNING">WARNING</SelectItem>
              <SelectItem value="CRITICAL">CRITICAL</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Time</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Severity</TableHead>
              <TableHead>Message</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((entry) => (
              <TableRow key={entry.id}>
                <TableCell>{new Date(entry.createdAt).toLocaleString()}</TableCell>
                <TableCell>{entry.category}</TableCell>
                <TableCell>{entry.severity}</TableCell>
                <TableCell>{entry.message}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>

        {status === "loading" ? <p className="text-sm text-muted-foreground">Refreshing audit logs...</p> : null}
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </CardContent>
    </Card>
  );
}
