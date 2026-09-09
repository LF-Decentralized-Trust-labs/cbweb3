// SPDX-License-Identifier: Apache-2.0

import {
  amountRefusalMessage,
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  parseBaseUnits,
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
  fiatUnitLabel,
  formatCeBM,
  formatCeBMDisplay,
  formatFiatUnits,
  getPaymentStatusLabel,
  getPaymentStatusVariant,
  normalizePaymentStatus,
  PaymentStatus,
} from "../types";

const shortHash = (value: string) =>
  value ? `${value.slice(0, 10)}...${value.slice(-8)}` : "-";

export function RedeemsPage() {
  const fetchAll = usePaymentStore((state) => state.fetchAll);
  const requestRedeem = usePaymentStore((state) => state.requestRedeem);
  const redeems = usePaymentStore((state) => state.redeems);
  const balance = usePaymentStore((state) => state.balance);
  const fiatBalance = usePaymentStore((state) => state.fiatBalance);
  const fiatDecimals = usePaymentStore((state) => state.fiatDecimals);
  const fiatSymbol = usePaymentStore((state) => state.fiatSymbol);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);
  const status = usePaymentStore((state) => state.status);
  const error = usePaymentStore((state) => state.error);

  const [amount, setAmount] = useState("0");

  // Parsed once. This field carries whole base units, the same unit as the
  // HTLC leg and the FX proposal, so the rule lives in @cbweb3/ui (lib/amount.ts)
  // rather than in an inline regex — and the field is plain text, because
  // <input type="number"> reads keystrokes through the browser locale.
  const parsedAmount = useMemo(() => parseBaseUnits(amount), [amount]);
  const [confirmRequest, setConfirmRequest] = useState(false);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  const pendingCount = useMemo(
    () =>
      redeems.filter(
        (item) => normalizePaymentStatus(item.status) === PaymentStatus.PENDING,
      ).length,
    [redeems],
  );

  const onSubmit = async () => {
    if (!parsedAmount.ok) {
      toast.error(amountRefusalMessage("Amount", parsedAmount.refusal));
      return;
    }

    try {
      const redeemId = await requestRedeem(amount);
      toast.success(`Redeem request submitted: ${redeemId}`);
      setAmount("0");
      setConfirmRequest(false);
    } catch (submitError) {
      toast.error(
        submitError instanceof Error
          ? submitError.message
          : "Unable to request redeem",
      );
    }
  };

  const onPrepareSubmit = () => {
    if (!parsedAmount.ok) {
      toast.error(amountRefusalMessage("Amount", parsedAmount.refusal));
      return;
    }
    setConfirmRequest(true);
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <BalanceWidget
          balance={balance}
          decimals={tCeBMDecimals}
          loading={status === "loading" && balance === null}
        />
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Fiat Reserve Balance</CardDescription>
            <CardTitle>
              {status === "loading" && fiatBalance === null
                ? "Loading..."
                : formatFiatUnits(fiatBalance ?? "0", fiatDecimals, fiatSymbol)}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Pending Redeems</CardDescription>
            <CardTitle>{pendingCount}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Redeem tCeBM (tCeBM to {fiatUnitLabel})</CardTitle>
          <CardDescription>
            The proxy executes Zeto transfer automatically before forwarding
            redeem request.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-2">
            <Label htmlFor="redeem-amount">Amount (tCeBM units)</Label>
            <Input
              id="redeem-amount"
              type="text"
              inputMode="numeric"
              autoComplete="off"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={onPrepareSubmit} disabled={status === "loading"}>
              Review Redeem Request
            </Button>
            <Button
              variant="outline"
              onClick={() => void fetchAll()}
              disabled={status === "loading"}
            >
              Refresh
            </Button>
          </div>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirmRequest ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Redeem Request</CardTitle>
            <CardDescription>
              {formatCeBMDisplay(amount, tCeBMDecimals)} will be submitted for central bank fiat
              reserve release approval.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button
              onClick={() => void onSubmit()}
              disabled={status === "loading"}
            >
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
          <CardTitle>Redeem Requests</CardTitle>
          <CardDescription>Total records: {redeems.length}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Zeto Transfer Tx Hash</TableHead>
                <TableHead>Redemption ID</TableHead>
                <TableHead>Rejection Reason</TableHead>
                <TableHead>Created At</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {redeems.map((redeem) => (
                <TableRow key={redeem.id}>
                  <TableCell className="font-medium">{redeem.id}</TableCell>
                  <TableCell>{formatCeBM(redeem.amount, tCeBMDecimals)}</TableCell>
                  <TableCell>
                    <Badge variant={getPaymentStatusVariant(redeem.status)}>
                      {getPaymentStatusLabel(redeem.status)}
                    </Badge>
                  </TableCell>
                  <TableCell title={redeem.zeto_transfer_tx_hash}>
                    {shortHash(redeem.zeto_transfer_tx_hash)}
                  </TableCell>
                  <TableCell title={redeem.fiat_mint_tx_hash}>
                    {shortHash(redeem.fiat_mint_tx_hash)}
                  </TableCell>
                  <TableCell>{redeem.rejection_reason || "-"}</TableCell>
                  <TableCell>
                    {new Date(redeem.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!redeems.length ? (
            <p className="pt-3 text-sm text-muted-foreground">
              No redeems found.
            </p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
