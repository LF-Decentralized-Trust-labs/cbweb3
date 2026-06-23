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
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useAmmV2Store } from "../features/amm/amm-v2.store";
import { usePaymentStore } from "../stores";

export function ApproveAmmPage() {
  const approveAmm = useAmmV2Store((state) => state.approveAmm);
  const ammApproved = useAmmV2Store((state) => state.ammApproved);
  const status = useAmmV2Store((state) => state.status);
  const error = useAmmV2Store((state) => state.error);

  const fetchAllPayments = usePaymentStore((state) => state.fetchAll);
  const balance = usePaymentStore((state) => state.balance);

  const [amount, setAmount] = useState("0");
  const [confirmApproval, setConfirmApproval] = useState(false);

  useEffect(() => {
    void fetchAllPayments();
  }, [fetchAllPayments]);

  const exceedsBalance = useMemo(() => {
    if (!amount || !balance) {
      return false;
    }

    try {
      return BigInt(amount) > BigInt(balance);
    } catch {
      return false;
    }
  }, [amount, balance]);

  const onPrepare = () => {
    if (!/^\d+$/.test(amount) || BigInt(amount) <= 0n) {
      toast.error("Amount must be a positive integer.");
      return;
    }

    setConfirmApproval(true);
  };

  const onApprove = async () => {
    try {
      await approveAmm(amount);
      toast.success("AMM approval submitted successfully.");
      setConfirmApproval(false);
    } catch (approveError) {
      toast.error(approveError instanceof Error ? approveError.message : "Unable to approve AMM");
    }
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Approve AMM Spending</CardTitle>
          <CardDescription>
            Authorize AMM spending as a standalone operation. The side is resolved server-side from gateway configuration.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="approve-amm-amount">Amount (tCeBM units)</Label>
            <Input
              id="approve-amm-amount"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              inputMode="numeric"
              pattern="[0-9]+"
            />
          </div>
          <div className="space-y-1 text-sm">
            <p>Current tCeBM balance: {balance ?? "0"}</p>
            {exceedsBalance ? <Badge variant="warning">Amount exceeds current balance (non-blocking warning)</Badge> : null}
          </div>
          <div className="flex gap-2">
            <Button type="button" onClick={onPrepare} disabled={status === "loading"}>
              Review Approval
            </Button>
            <Button type="button" variant="outline" onClick={() => void fetchAllPayments()} disabled={status === "loading"}>
              Refresh Balance
            </Button>
          </div>
          {ammApproved ? <Badge variant="success">AMM approval confirmed in current session.</Badge> : null}
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirmApproval ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm AMM Approval</CardTitle>
            <CardDescription>You are about to approve {amount} tCeBM for AMM usage.</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button type="button" onClick={() => void onApprove()} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Confirm Approval"}
            </Button>
            <Button type="button" variant="outline" onClick={() => setConfirmApproval(false)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
