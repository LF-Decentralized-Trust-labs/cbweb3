import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  DatePicker,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useAuditLogs } from "../hooks";
import type { AuditCategory, AuditSeverity } from "../types";

export function AuditPage() {
  const { logs, fetch, status } = useAuditLogs();
  const [category, setCategory] = useState<AuditCategory | "ALL">("ALL");
  const [severity, setSeverity] = useState<AuditSeverity | "ALL">("ALL");
  const [dateFrom, setDateFrom] = useState<Date | undefined>(undefined);
  const [dateTo, setDateTo] = useState<Date | undefined>(undefined);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const onApply = async () => {
    await fetch({
      category,
      severity,
      dateFrom: dateFrom?.toISOString(),
      dateTo: dateTo?.toISOString(),
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Governance Audit Trail</CardTitle>
        <CardDescription>Immutable history of governance actions and outcomes.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 md:grid-cols-4">
          <Select value={category} onValueChange={(value) => setCategory(value as AuditCategory | "ALL")}>
            <SelectTrigger>
              <SelectValue placeholder="Category" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All categories</SelectItem>
              <SelectItem value="REGISTRY">Registry</SelectItem>
              <SelectItem value="CIRCUIT_BREAKER">Circuit Breaker</SelectItem>
              <SelectItem value="FREEZE">Freeze</SelectItem>
              <SelectItem value="PARAMETER">Parameter</SelectItem>
              <SelectItem value="AUTH">Auth</SelectItem>
            </SelectContent>
          </Select>

          <Select value={severity} onValueChange={(value) => setSeverity(value as AuditSeverity | "ALL")}>
            <SelectTrigger>
              <SelectValue placeholder="Severity" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All severities</SelectItem>
              <SelectItem value="INFO">Info</SelectItem>
              <SelectItem value="WARNING">Warning</SelectItem>
              <SelectItem value="CRITICAL">Critical</SelectItem>
            </SelectContent>
          </Select>

          <DatePicker value={dateFrom} onChange={setDateFrom} placeholder="From date" />
          <DatePicker value={dateTo} onChange={setDateTo} placeholder="To date" />
        </div>

        <Button onClick={() => void onApply()} disabled={status === "loading"}>
          {status === "loading" ? "Applying..." : "Apply Filters"}
        </Button>

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
            {logs.map((log) => (
              <TableRow key={log.id}>
                <TableCell>{new Date(log.createdAt).toLocaleString()}</TableCell>
                <TableCell>{log.actor}</TableCell>
                <TableCell>{log.action}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{log.category}</Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={log.severity === "CRITICAL" ? "destructive" : log.severity === "WARNING" ? "warning" : "secondary"}>
                    {log.severity}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={log.outcome === "SUCCESS" ? "default" : "destructive"}>{log.outcome}</Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
