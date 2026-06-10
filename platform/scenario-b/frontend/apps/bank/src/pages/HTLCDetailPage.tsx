import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  toast,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { Link, Navigate, useParams } from "react-router-dom";
import { BalanceWidget } from "../components/common/BalanceWidget";
import { useHTLCStatus } from "../hooks/useHTLCStatus";
import { useTimelockCountdown } from "../hooks/useTimelockCountdown";
import { useHtlcStore, usePaymentStore } from "../stores";

const normalizeState = (state: string) => state.replace("HTLC_STATE_", "");

const isLockedState = (state: string) => normalizeState(state) === "LOCKED";
const isSettledState = (state: string) => normalizeState(state) === "SETTLED";
const isRefundedState = (state: string) => normalizeState(state) === "REFUNDED";
const isInProgressState = (state: string) =>
  normalizeState(state) === "SETTLING" || normalizeState(state) === "REFUNDING";

const statusVariant = (state: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  if (isLockedState(state)) return "warning";
  if (isSettledState(state)) return "success";
  if (isRefundedState(state)) return "destructive";
  if (isInProgressState(state)) return "default";
  return "outline";
};

export function HTLCDetailPage() {
  const params = useParams<{ contractId: string }>();
  const contractId = params.contractId;
  const safeContractId = contractId ?? "";
  const settle = useHtlcStore((state) => state.settle);
  const refund = useHtlcStore((state) => state.refund);
  const getStatus = useHtlcStore((state) => state.getStatus);
  const storeStatus = useHtlcStore((state) => state.status);
  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const balance = usePaymentStore((state) => state.balance);
  const paymentStatus = usePaymentStore((state) => state.status);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);

  const [confirmSettle, setConfirmSettle] = useState(false);
  const [confirmRefund, setConfirmRefund] = useState(false);

  const { htlc, loading } = useHTLCStatus(safeContractId);
  const countdown = useTimelockCountdown(htlc?.time_lock ?? 0);

  useEffect(() => {
    void fetchPayments();
  }, [fetchPayments]);

  if (!contractId) {
    return <Navigate to="/htlc" replace />;
  }

  const canSettle = Boolean(htlc && isLockedState(htlc.state) && htlc.secret);
  const canRefund = Boolean(htlc && isLockedState(htlc.state) && countdown.isExpired);
  const isFinalState = Boolean(htlc && (isSettledState(htlc.state) || isRefundedState(htlc.state)));

  const waitForState = async (targetState: "HTLC_STATE_SETTLED" | "HTLC_STATE_REFUNDED", timeoutMs = 30_000, stepMs = 2_000) => {
    const startedAt = Date.now();
    while (Date.now() - startedAt < timeoutMs) {
      const latest = await getStatus(contractId);
      if (normalizeState(latest.state) === normalizeState(targetState)) {
        return true;
      }
      await new Promise((resolve) => setTimeout(resolve, stepMs));
    }
    return false;
  };

  const onSettle = async () => {
    if (!htlc?.secret) {
      toast.error("Completion code unavailable. Settlement cannot be finalized.");
      return;
    }
    try {
      await settle(contractId, htlc.secret);
      toast.success("Settlement completed successfully.");
      setConfirmSettle(false);
    } catch (error) {
      const isTimeout = error instanceof Error && /timeout|ECONNABORTED/i.test(error.message);
      if (isTimeout) {
        try {
          const settled = await waitForState("HTLC_STATE_SETTLED");
          if (settled) {
            toast.success("Settlement completed successfully.");
            setConfirmSettle(false);
            return;
          }
        } catch {
          // Fall through to user-facing error below.
        }
      }
      toast.error(error instanceof Error ? error.message : "Unable to complete settlement.");
    }
  };

  const onRefund = async () => {
    try {
      await refund(contractId);
      toast.success("Settlement revoked. Funds returned.");
      setConfirmRefund(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to revoke settlement.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Settlement Details</h1>
          <p className="text-sm text-muted-foreground">Contract {contractId}</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/htlc">Back to history</Link>
        </Button>
      </div>

      <BalanceWidget balance={balance} loading={paymentStatus === "loading" && balance === null} decimals={tCeBMDecimals} />

      {loading ? <p className="text-sm text-muted-foreground">Loading contract details...</p> : null}

      {htlc ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Settlement State</CardTitle>
              <CardDescription>Current settlement lifecycle and metadata.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Badge variant={statusVariant(htlc.state)}>
                {(() => {
                  const s = normalizeState(htlc.state);
                  if (s === "LOCKED") return "Pending Settlement";
                  if (s === "SETTLING") return "Processing";
                  if (s === "SETTLED") return "Settled";
                  if (s === "REFUNDING") return "Revoking";
                  if (s === "REFUNDED") return "Revoked";
                  return s;
                })()}
              </Badge>
              <p className="text-sm">Sender: {htlc.sender}</p>
              <p className="text-sm">Receiver: {htlc.receiver}</p>
              <p className="text-sm">Settlement Code: {htlc.hash_lock}</p>
              <p className="text-sm">Settlement Expiry: {new Date(htlc.time_lock * 1000).toLocaleString()}</p>
              <p className="text-sm">Completion Code: {htlc.secret ? `${htlc.secret.slice(0, 8)}...` : "not available"}</p>
            </CardContent>
          </Card>

          {isLockedState(htlc.state) ? (
            <Card>
              <CardHeader>
                <CardTitle>Settlement Window</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-lg font-semibold">{countdown.display}</p>
                <p className="text-xs text-muted-foreground">
                  {countdown.isExpired ? "Settlement window has expired. Revocation is available." : "Settlement window is active."}
                </p>
              </CardContent>
            </Card>
          ) : null}

          {isInProgressState(htlc.state) ? (
            <Card>
              <CardHeader>
                <CardTitle>Operation In Progress</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">
                  {normalizeState(htlc.state) === "SETTLING"
                    ? "Settlement is being processed. The page will update automatically."
                    : "Revocation is being processed. The page will update automatically."}
                </p>
              </CardContent>
            </Card>
          ) : null}

          <Card>
            <CardHeader>
              <CardTitle>Actions</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Button disabled={!canSettle} onClick={() => setConfirmSettle(true)}>
                Complete Settlement
              </Button>
              <Button variant="outline" disabled={!canRefund} onClick={() => setConfirmRefund(true)}>
                Revoke Settlement
              </Button>
            </CardContent>
          </Card>

          {confirmSettle && !isFinalState ? (
            <Card>
              <CardHeader>
                <CardTitle>Confirm Settlement Completion</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <p className="text-sm">This will finalize the cross-border transfer. This action is irreversible.</p>
                <div className="flex gap-2">
                  <Button onClick={() => void onSettle()} disabled={storeStatus === "loading"}>
                    {storeStatus === "loading" ? "Submitting..." : "Confirm Settlement"}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmSettle(false)}>
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          ) : null}

          {confirmRefund && !isFinalState ? (
            <Card>
              <CardHeader>
                <CardTitle>Confirm Revocation</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <p className="text-sm">This will return the reserved funds to the originating account.</p>
                <div className="flex gap-2">
                  <Button onClick={() => void onRefund()} disabled={storeStatus === "loading"}>
                    {storeStatus === "loading" ? "Submitting..." : "Confirm Revocation"}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmRefund(false)}>
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
