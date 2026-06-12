import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
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
  const { logs, page, limit, lastDecrypted, status, error, refreshLogs, decryptTransaction } = useAuditStore();
  const [txHash, setTxHash] = useState("0x8f41b290d90e10a89c90f6d20d9eae1288a8ff1f9d");
  const [viewKey, setViewKey] = useState("regulatory-view-key-local-session");
  const [reason, setReason] = useState("AML investigation review");
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
          <CardTitle>Compliance & Audit Vault</CardTitle>
          <CardDescription>Use regulatory view keys to decrypt shielded transactions for forensic review.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 lg:grid-cols-2">
          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="tx-hash">Transaction hash</Label>
              <Input id="tx-hash" value={txHash} onChange={(event) => setTxHash(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="view-key">Regulatory view key (in-memory only)</Label>
              <Input id="view-key" type="password" value={viewKey} onChange={(event) => setViewKey(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="reason">Reason</Label>
              <Textarea id="reason" value={reason} onChange={(event) => setReason(event.target.value)} />
            </div>
            <Button
              disabled={status === "loading" || txHash.length < 8 || viewKey.length < 8 || reason.length < 5}
              onClick={async () => {
                await decryptTransaction({ txHash, viewKey, reason });
                toast("Transaction decrypted for active session");
              }}
            >
              Decrypt Transaction
            </Button>
            <p className="text-xs text-muted-foreground">Sensitive decrypted values are not persisted to browser localStorage/sessionStorage.</p>
          </div>

          <div className="rounded-md border border-border bg-muted/20 p-3">
            <h3 className="mb-2 text-sm font-semibold">Decryption Result</h3>
            {lastDecrypted ? (
              <dl className="space-y-1 text-sm">
                <div className="flex justify-between gap-4"><dt>Tx</dt><dd className="font-mono text-xs">{lastDecrypted.txHash}</dd></div>
                <div className="flex justify-between gap-4"><dt>Amount</dt><dd>{lastDecrypted.amount} {lastDecrypted.currency}</dd></div>
                <div className="flex justify-between gap-4"><dt>Sender</dt><dd className="font-mono text-xs">{lastDecrypted.sender}</dd></div>
                <div className="flex justify-between gap-4"><dt>Receiver</dt><dd className="font-mono text-xs">{lastDecrypted.receiver}</dd></div>
              </dl>
            ) : (
              <p className="text-sm text-muted-foreground">No decrypted payload in session yet.</p>
            )}
            {error ? <p className="mt-2 text-sm text-destructive">{error}</p> : null}
          </div>
        </CardContent>
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
                  <TableCell className="max-w-[140px] truncate" title={log.actor}>{log.actor}</TableCell>
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
