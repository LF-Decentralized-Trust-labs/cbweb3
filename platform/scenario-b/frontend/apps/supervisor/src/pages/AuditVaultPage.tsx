// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
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
import { useEffect, useState } from "react";
import { CopyableValue } from "../components/common/CopyableValue";
import { ZKPointerPanel } from "../components/supervisor/ZKPointerPanel";
import { useAuditStore } from "../stores";

const SEVERITY_VARIANTS: Record<string, "destructive" | "warning" | "secondary" | "default"> = {
  CRITICAL: "destructive",
  HIGH: "destructive",
  MEDIUM: "warning",
  LOW: "secondary",
  INFO: "default",
};

export function AuditVaultPage() {
  const { logs, page, limit, status, error, refreshLogs } = useAuditStore();
  const [severityFilter, setSeverityFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("");

  useEffect(() => {
    void refreshLogs();
  }, [refreshLogs]);

  const applyFilters = () => {
    void refreshLogs({
      severity: severityFilter || undefined,
      category: categoryFilter || undefined,
    });
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Privacy Notice · Scenario B</CardTitle>
          <CardDescription>
            Scenario B uses Zeto ZKP commitments — transaction amounts and parties are never revealed
            on-chain. There is no regulatory view-key decrypt in this scenario. Use ZK-Pointer
            verification below to confirm that a shielded transfer commitment was validly issued.
          </CardDescription>
        </CardHeader>
      </Card>

      <ZKPointerPanel />

      <Card>
        <CardHeader>
          <CardTitle>Immutable Audit Logs</CardTitle>
          <CardDescription>Read-only compliance audit logs from the backend. Page {page} · {limit} per page.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap gap-2">
            <Input
              className="w-36"
              placeholder="Severity (e.g. CRITICAL)"
              value={severityFilter}
              onChange={(e) => setSeverityFilter(e.target.value)}
            />
            <Input
              className="w-36"
              placeholder="Category"
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
            />
            <Button variant="secondary" size="sm" onClick={applyFilters} disabled={status === "loading"}>
              Filter
            </Button>
            <Button variant="ghost" size="sm" onClick={() => { setSeverityFilter(""); setCategoryFilter(""); void refreshLogs(); }} disabled={status === "loading"}>
              Clear
            </Button>
          </div>

          {status === "error" && error ? (
            <p className="text-sm text-destructive">{error}</p>
          ) : null}

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Actor</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Outcome</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.length === 0 && status !== "loading" ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground">No audit logs found.</TableCell>
                </TableRow>
              ) : null}
              {logs.map((log) => (
                <TableRow key={log.log_id}>
                  <TableCell>{new Date(log.timestamp).toLocaleString()}</TableCell>
                  <TableCell className="max-w-[180px]"><CopyableValue value={log.actor} truncate={18} /></TableCell>
                  <TableCell>{log.action}</TableCell>
                  <TableCell>{log.category}</TableCell>
                  <TableCell>
                    <Badge variant={SEVERITY_VARIANTS[log.severity] ?? "default"}>{log.severity}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={log.outcome === "SUCCESS" ? "success" : "destructive"}>{log.outcome}</Badge>
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
