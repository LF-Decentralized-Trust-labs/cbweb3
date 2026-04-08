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
import { Link, useNavigate } from "react-router-dom";
import { BalanceWidget } from "../components/common/BalanceWidget";
import type { HTLCLock, HTLCSearchState } from "../types";
import { useHtlcStore, usePaymentStore } from "../stores";

type FilterState = "ALL" | HTLCSearchState;

const shortState = (value: string) => value.replace("HTLC_STATE_", "");

const statusVariant = (state: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  if (state === "HTLC_STATE_LOCKED") return "warning";
  if (state === "HTLC_STATE_SETTLED") return "success";
  if (state === "HTLC_STATE_REFUNDED") return "destructive";
  return "outline";
};

export function HTLCHistoryPage() {
  const navigate = useNavigate();
  const search = useHtlcStore((state) => state.search);
  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const balance = usePaymentStore((state) => state.balance);
  const paymentStatus = usePaymentStore((state) => state.status);

  const [stateFilter, setStateFilter] = useState<FilterState>("ALL");
  const [agreementIdFilter, setAgreementIdFilter] = useState("");
  const [locks, setLocks] = useState<HTLCLock[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const response = await search({
        state: stateFilter === "ALL" ? undefined : stateFilter,
        agreement_id: agreementIdFilter.trim() || undefined,
      });
      setLocks(response.locks);
      setTotal(response.total);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to load HTLC history.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    void fetchPayments();
  }, [fetchPayments]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">HTLC History</h1>
          <p className="text-sm text-muted-foreground">Search and inspect cross-spoke HTLC contracts.</p>
        </div>
        <Button asChild>
          <Link to="/htlc/new">New HTLC Lock</Link>
        </Button>
      </div>

      <BalanceWidget balance={balance} loading={paymentStatus === "loading" && balance === null} />

      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>Filter by status or agreement ID.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-3">
          <Select value={stateFilter} onValueChange={(value) => setStateFilter(value as FilterState)}>
            <SelectTrigger>
              <SelectValue placeholder="Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">All</SelectItem>
              <SelectItem value="LOCKED">LOCKED</SelectItem>
              <SelectItem value="SETTLED">SETTLED</SelectItem>
              <SelectItem value="REFUNDED">REFUNDED</SelectItem>
            </SelectContent>
          </Select>
          <Input
            placeholder="Agreement ID"
            value={agreementIdFilter}
            onChange={(event) => setAgreementIdFilter(event.target.value)}
          />
          <Button onClick={() => void load()} disabled={loading}>
            {loading ? "Searching..." : "Search"}
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
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {locks.map((lock) => (
                <TableRow key={lock.contract_id}>
                  <TableCell className="font-medium">{lock.contract_id}</TableCell>
                  <TableCell>{lock.sender}</TableCell>
                  <TableCell>{lock.receiver}</TableCell>
                  <TableCell>
                    <Badge variant={statusVariant(lock.state)}>{shortState(lock.state)}</Badge>
                  </TableCell>
                  <TableCell>{new Date(lock.time_lock * 1000).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    <Button size="sm" variant="outline" onClick={() => navigate(`/htlc/${lock.contract_id}`)}>
                      View
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!locks.length ? <p className="pt-3 text-sm text-muted-foreground">No HTLCs found.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
