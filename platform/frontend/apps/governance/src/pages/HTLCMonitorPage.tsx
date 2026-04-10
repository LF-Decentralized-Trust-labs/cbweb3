import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
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
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import type { HTLCLock, HTLCSearchState } from "../types";
import { useHtlcMonitorStore } from "../stores";

type FilterState = "ALL" | HTLCSearchState;

const statusVariant = (state: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  if (state === "HTLC_STATE_LOCKED") return "warning";
  if (state === "HTLC_STATE_SETTLED") return "success";
  if (state === "HTLC_STATE_REFUNDED") return "destructive";
  if (state === "HTLC_STATE_SETTLING" || state === "HTLC_STATE_REFUNDING") return "default";
  return "outline";
};

export function HTLCMonitorPage() {
  const { locks, total, status, error, fetch, getDetail } = useHtlcMonitorStore();

  const [stateFilter, setStateFilter] = useState<FilterState>("ALL");
  const [agreementIdFilter, setAgreementIdFilter] = useState("");
  const [detail, setDetail] = useState<HTLCLock | null>(null);

  const load = async () => {
    await fetch({
      state: stateFilter === "ALL" ? undefined : stateFilter,
      agreement_id: agreementIdFilter.trim() || undefined,
    });
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openDetail = async (contractId: string) => {
    try {
      const data = await getDetail(contractId);
      setDetail(data);
    } catch (fetchError) {
      toast.error(fetchError instanceof Error ? fetchError.message : "Unable to load PvP settlement details.");
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-xl font-semibold">PvP Settlement Monitor</h1>
        <p className="text-sm text-muted-foreground">Read-only oversight for lock, settle, and refund lifecycle events.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>Refine monitor results by state or agreement ID.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-3">
          <Select value={stateFilter} onValueChange={(value) => setStateFilter(value as FilterState)}>
            <SelectTrigger>
              <SelectValue placeholder="State" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All</SelectItem>
              <SelectItem value="LOCKED">LOCKED</SelectItem>
              <SelectItem value="SETTLING">SETTLING</SelectItem>
              <SelectItem value="SETTLED">SETTLED</SelectItem>
              <SelectItem value="REFUNDING">REFUNDING</SelectItem>
              <SelectItem value="REFUNDED">REFUNDED</SelectItem>
            </SelectContent>
          </Select>
          <Input
            placeholder="Agreement ID"
            value={agreementIdFilter}
            onChange={(event) => setAgreementIdFilter(event.target.value)}
          />
          <Button onClick={() => void load()} disabled={status === "loading"}>
            {status === "loading" ? "Loading..." : "Search"}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Contracts</CardTitle>
          <CardDescription>Total results: {total}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Contract</TableHead>
                <TableHead>Sender</TableHead>
                <TableHead>Receiver</TableHead>
                <TableHead>State</TableHead>
                <TableHead>Time Lock</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {locks.map((lock) => (
                <TableRow key={lock.contract_id}>
                  <TableCell className="font-medium">{lock.contract_id}</TableCell>
                  <TableCell>{lock.sender}</TableCell>
                  <TableCell>{lock.receiver}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(lock.state)}>{lock.state.replace("HTLC_STATE_", "")}</Badge>
                  </TableCell>
                  <TableCell>{new Date(lock.time_lock * 1000).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" size="sm" onClick={() => void openDetail(lock.contract_id)}>
                      View details
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!locks.length ? <p className="pt-3 text-sm text-muted-foreground">No PvP settlement records found.</p> : null}
          {error ? <p className="pt-3 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {detail ? (
        <Card>
          <CardHeader>
            <CardTitle>Contract Detail: {detail.contract_id}</CardTitle>
            <CardDescription>Governance read-only detail view.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="text-sm">Sender: {detail.sender}</p>
            <p className="text-sm">Receiver: {detail.receiver}</p>
            <p className="text-sm">Hash lock: {detail.hash_lock}</p>
            <p className="text-sm">Time lock: {new Date(detail.time_lock * 1000).toLocaleString()}</p>
            <p className="text-sm">State: {detail.state.replace("HTLC_STATE_", "")}</p>
            <p className="text-sm">Zeto lock ref: {detail.zeto_lock_ref || "-"}</p>
            <p className="text-sm">Secret: {detail.secret ? "[hidden by policy]" : "-"}</p>
            <Button variant="outline" onClick={() => setDetail(null)}>
              Close
            </Button>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
