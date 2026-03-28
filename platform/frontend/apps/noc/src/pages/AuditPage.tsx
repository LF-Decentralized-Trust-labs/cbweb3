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

export function AuditPage() {
  const { logs, fetch } = useAuditLogs();
  const [search, setSearch] = useState("");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const filtered = useMemo(() => {
    if (!search.trim()) return logs;
    const term = search.toLowerCase();
    return logs.filter(
      (record) =>
        record.component.toLowerCase().includes(term) ||
        record.message.toLowerCase().includes(term) ||
        record.severity.toLowerCase().includes(term),
    );
  }, [logs, search]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Audit Trail</CardTitle>
        <CardDescription>Immutable operator actions and control-plane decisions.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Input
          placeholder="Search actor, action, target"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Timestamp</TableHead>
              <TableHead>Component</TableHead>
              <TableHead>Severity</TableHead>
              <TableHead>Message</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((record) => (
              <TableRow key={record.id}>
                <TableCell>{new Date(record.createdAt).toLocaleString()}</TableCell>
                <TableCell>{record.component}</TableCell>
                <TableCell>
                  <Badge variant={record.severity === "CRITICAL" ? "destructive" : record.severity === "WARNING" ? "warning" : "secondary"}>
                    {record.severity}
                  </Badge>
                </TableCell>
                <TableCell>{record.message}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
