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
  fiatUnitLabel,
  formatFiatUnits,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
} from "../types";

const shortHash = (value: string) => (value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-");

export function DepositsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const registerDeposit = usePaymentStore((state) => state.registerDeposit);
  const deposits = usePaymentStore((state) => state.deposits);
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
    () => deposits.filter((item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING).length,
    [deposits],
  );

  const onSubmit = async () => {
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return;
    }

    try {
      const depositId = await registerDeposit(amount);
      toast.success(`Issuance request submitted: ${depositId}`);
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(submitError instanceof Error ? submitError.message : "Unable to submit issuance request");
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
            <CardDescription>Pending Issuance Requests</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Request Token Issuance</CardTitle>
          <CardDescription>Submit fiat collateral proof to request tCeBM issuance by the central bank.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="deposit-amount">Amount ({fiatUnitLabel})</Label>
            <Input
              id="deposit-amount"
              type="number"
              min="0"
              step="1"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Issuance Request
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
            <CardTitle>Confirm Issuance Request</CardTitle>
            <CardDescription>{formatFiatUnits(amount)} will be submitted for central bank approval.</CardDescription>
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
          <CardTitle>Issuance Requests</CardTitle>
          <CardDescription>Total records: {deposits.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Fiat Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Issuance Reference</TableHead>
                <TableHead>Rejection Reason</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deposits.map((deposit) => (
                <TableRow key={deposit.id}>
                  <TableCell className="font-medium">{deposit.id}</TableCell>
                  <TableCell>{formatFiatUnits(deposit.amount)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(deposit.status)}>{getPaymentStatusLabel(deposit.status)}</Badge>
                  </TableCell>
                  <TableCell title={deposit.mint_tx_hash}>{shortHash(deposit.mint_tx_hash)}</TableCell>
                  <TableCell>{deposit.rejection_reason || "-"}</TableCell>
                  <TableCell>{new Date(deposit.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!deposits.length ? <p className="pt-3 text-sm text-muted-foreground">No issuance requests found.</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
