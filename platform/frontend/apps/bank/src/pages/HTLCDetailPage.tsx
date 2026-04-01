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
import { useState } from "react";
import { Link, Navigate, useParams } from "react-router-dom";
import { useHTLCStatus } from "../hooks/useHTLCStatus";
import { useTimelockCountdown } from "../hooks/useTimelockCountdown";
import { useHtlcStore } from "../stores";

const statusVariant = (state: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  if (state === "HTLC_STATE_LOCKED") return "warning";
  if (state === "HTLC_STATE_SETTLED") return "success";
  if (state === "HTLC_STATE_REFUNDED") return "destructive";
  return "outline";
};

export function HTLCDetailPage() {
  const params = useParams<{ contractId: string }>();
  const contractId = params.contractId;
  const safeContractId = contractId ?? "";
  const settle = useHtlcStore((state) => state.settle);
  const refund = useHtlcStore((state) => state.refund);
  const storeStatus = useHtlcStore((state) => state.status);

  const [confirmSettle, setConfirmSettle] = useState(false);
  const [confirmRefund, setConfirmRefund] = useState(false);

  const { htlc, loading } = useHTLCStatus(safeContractId);
  const countdown = useTimelockCountdown(htlc?.time_lock ?? 0);

  if (!contractId) {
    return <Navigate to="/htlc" replace />;
  }

  const canSettle = Boolean(htlc && htlc.state === "HTLC_STATE_LOCKED" && htlc.secret);
  const canRefund = Boolean(htlc && htlc.state === "HTLC_STATE_LOCKED" && countdown.isExpired);

  const onSettle = async () => {
    if (!htlc?.secret) {
      toast.error("Secret not available for settle.");
      return;
    }
    try {
      await settle(contractId, htlc.secret);
      toast.success("HTLC settled successfully.");
      setConfirmSettle(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to settle HTLC.");
    }
  };

  const onRefund = async () => {
    try {
      await refund(contractId);
      toast.success("HTLC refunded successfully.");
      setConfirmRefund(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to refund HTLC.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">HTLC Details</h1>
          <p className="text-sm text-muted-foreground">Contract {contractId}</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/htlc">Back to history</Link>
        </Button>
      </div>

      {loading ? <p className="text-sm text-muted-foreground">Loading contract details...</p> : null}

      {htlc ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Contract State</CardTitle>
              <CardDescription>Current lock lifecycle and metadata.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Badge variant={statusVariant(htlc.state)}>{htlc.state.replace("HTLC_STATE_", "")}</Badge>
              <p className="text-sm">Sender: {htlc.sender}</p>
              <p className="text-sm">Receiver: {htlc.receiver}</p>
              <p className="text-sm">Hash lock: {htlc.hash_lock}</p>
              <p className="text-sm">Time lock: {new Date(htlc.time_lock * 1000).toLocaleString()}</p>
              <p className="text-sm">Zeto lock ref: {htlc.zeto_lock_ref || "-"}</p>
              <p className="text-sm">Secret: {htlc.secret ? `${htlc.secret.slice(0, 8)}...` : "not available"}</p>
            </CardContent>
          </Card>

          {htlc.state === "HTLC_STATE_LOCKED" ? (
            <Card>
              <CardHeader>
                <CardTitle>Timelock Countdown</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-lg font-semibold">{countdown.display}</p>
                <p className="text-xs text-muted-foreground">
                  {countdown.isExpired ? "Lock expired. Refund is available." : "Lock active. Await settle or expiration."}
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
                Settle HTLC
              </Button>
              <Button variant="outline" disabled={!canRefund} onClick={() => setConfirmRefund(true)}>
                Refund HTLC
              </Button>
            </CardContent>
          </Card>

          {confirmSettle ? (
            <Card>
              <CardHeader>
                <CardTitle>Confirm Settle</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <p className="text-sm">This action is irreversible and reveals the secret.</p>
                <div className="flex gap-2">
                  <Button onClick={() => void onSettle()} disabled={storeStatus === "loading"}>
                    {storeStatus === "loading" ? "Submitting..." : "Confirm Settle"}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmSettle(false)}>
                    Cancel
                  </Button>
                </div>
              </CardContent>
            </Card>
          ) : null}

          {confirmRefund ? (
            <Card>
              <CardHeader>
                <CardTitle>Confirm Refund</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <p className="text-sm">This will return the locked funds to the original sender.</p>
                <div className="flex gap-2">
                  <Button onClick={() => void onRefund()} disabled={storeStatus === "loading"}>
                    {storeStatus === "loading" ? "Submitting..." : "Confirm Refund"}
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
