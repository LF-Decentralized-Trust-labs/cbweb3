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
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { BalanceWidget } from "../components/common/BalanceWidget";
import { usePaymentStore } from "../stores";
import {
  PaymentStatus,
  formatCeBM,
  formatFiatUnits,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
} from "../types";

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

export function EscrowsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const requestEscrow = usePaymentStore((state) => state.requestEscrow);
  const escrows = usePaymentStore((state) => state.escrows);
  const balance = usePaymentStore((state) => state.balance);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);

  const [amount, setAmount] = useState("0");
  const [confirmRequest, setConfirmRequest] = useState(false);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const pendingCount = useMemo(
    () => escrows.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [escrows],
  );

  const onSubmit = async () => {
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return;
    }

    try {
      const escrowId = await requestEscrow(amount);
      toast.success(`Escrow request submitted: ${escrowId}`);
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(submitError instanceof Error ? submitError.message : "Unable to request escrow");
    }
  };

  const onPrepareSubmit = () => {
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return;
    }
    setConfirmRequest(true);
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <BalanceWidget balance={balance} loading={status === "loading" && balance === null} />
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fiat Reserve Balance</CardDescription>
            <CardTitle>{status === "loading" && fiatBalance === null ? "Loading..." : formatFiatUnits(fiatBalance ?? "0")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">Mirrors commercial bank fiat reserves</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Escrows</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="warning">Awaiting central bank tokenization</Badge>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Request Escrow (fCeBM to tCeBM)</CardTitle>
          <CardDescription>Create escrow requests after deposit approval and fiat exchange.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="escrow-amount">Amount (CeBM units)</Label>
            <Input
              id="escrow-amount"
              type="number"
              min="0"
              step="1"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
          <p className="text-xs text-muted-foreground">Only approved deposits should be tokenized through this operation.</p>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Escrow Request
            </Button>
            <Button variant="outline" onClick={() => void fetchAll()} disabled={status === "loading"}>
              Refresh
            </Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirmRequest ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Escrow Request</CardTitle>
            <CardDescription>{formatCeBM(amount)} will be submitted for central bank tokenization approval.</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button onClick={() => void onSubmit()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Confirm Request"}
            </Button>
            <Button variant="outline" onClick={() => setConfirmRequest(false)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Escrow Requests</CardTitle>
          <CardDescription>Total records: {escrows.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Burn Tx Hash</TableHead>
                <TableHead>Mint Tx Hash</TableHead>
                <TableHead>Rejection Reason</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {escrows.map((escrow) => (
                <TableRow key={escrow.id}>
                  <TableCell className="font-medium">{escrow.id}</TableCell>
                  <TableCell>{formatCeBM(escrow.amount)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(escrow.status)}>{getPaymentStatusLabel(escrow.status)}</Badge>
                  </TableCell>
                  <TableCell title={escrow.burn_tx_hash}>{shortHash(escrow.burn_tx_hash)}</TableCell>
                  <TableCell title={escrow.mint_tx_hash}>{shortHash(escrow.mint_tx_hash)}</TableCell>
                  <TableCell>{escrow.rejection_reason || "-"}</TableCell>
                  <TableCell>{new Date(escrow.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!escrows.length ? <p className="pt-3 text-sm text-muted-foreground">No escrows found.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
