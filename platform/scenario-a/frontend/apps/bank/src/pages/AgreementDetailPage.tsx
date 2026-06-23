// SPDX-License-Identifier: Apache-2.0

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
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { useFxAgreementStore } from "../stores/fx-agreement.store";
import type { FXAgreementState } from "../types";

const shortState = (state: FXAgreementState) => {
  const s = state.replace("FX_STATE_", "");
  if (s === "PROPOSED") return "Proposed";
  if (s === "ACCEPTED") return "Accepted";
  if (s === "REJECTED") return "Rejected";
  if (s === "CANCELLED") return "Cancelled";
  if (s === "SETTLED") return "Settled";
  return s;
};

const statusVariant = (state: FXAgreementState): "warning" | "success" | "destructive" | "outline" => {
  if (state === "FX_STATE_PROPOSED") return "warning";
  if (state === "FX_STATE_ACCEPTED") return "success";
  if (state === "FX_STATE_REJECTED") return "destructive";
  if (state === "FX_STATE_CANCELLED") return "outline";
  if (state === "FX_STATE_SETTLED") return "success";
  return "outline";
};

export function AgreementDetailPage() {
  const params = useParams<{ tradeId: string }>();
  const tradeId = params.tradeId;

  const navigate = useNavigate();
  const getAgreement = useFxAgreementStore((s) => s.getAgreement);
  const currentAgreement = useFxAgreementStore((s) => s.currentAgreement);
  const accept = useFxAgreementStore((s) => s.accept);
  const reject = useFxAgreementStore((s) => s.reject);
  const cancel = useFxAgreementStore((s) => s.cancel);
  const storeStatus = useFxAgreementStore((s) => s.status);

  const [confirmAction, setConfirmAction] = useState<"accept" | "reject" | "cancel" | null>(null);

  useEffect(() => {
    if (tradeId) {
      void getAgreement(tradeId);
    }
  }, [tradeId, getAgreement]);

  if (!tradeId) {
    return <Navigate to="/agreements" replace />;
  }

  const agreement = currentAgreement;
  const isProposed = agreement?.state === "FX_STATE_PROPOSED";
  const isAccepted = agreement?.state === "FX_STATE_ACCEPTED";

  const onAction = async (action: "accept" | "reject" | "cancel") => {
    try {
      if (action === "accept") await accept(tradeId);
      else if (action === "reject") await reject(tradeId);
      else if (action === "cancel") await cancel(tradeId);
      toast.success(`Agreement ${action}ed successfully.`);
      setConfirmAction(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : `Unable to ${action} agreement.`);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">Agreement Details</h1>
          <p className="text-sm text-muted-foreground">Trade ID: {tradeId}</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/agreements">Back to agreements</Link>
        </Button>
      </div>

      {storeStatus === "loading" && !agreement ? (
        <p className="text-sm text-muted-foreground">Loading agreement details...</p>
      ) : null}

      {agreement ? (
        <>
          <Card>
            <CardHeader>
              <CardTitle>Status</CardTitle>
            </CardHeader>
            <CardContent>
              <Badge variant={statusVariant(agreement.state)}>{shortState(agreement.state)}</Badge>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Parties</CardTitle>
              <CardDescription>Participants in this trade agreement.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <p className="text-sm">Originator: {agreement.originator ?? agreement.counterparty_b}</p>
              <p className="text-sm">Counterparty: {agreement.counterparty_b}</p>
              <p className="text-sm">Settlement Agent: {agreement.settlement_agent}</p>
              <p className="text-sm">Custodian: {agreement.custodian}</p>
              <p className="text-sm">Beneficiary: {agreement.beneficiary}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Trade Terms</CardTitle>
              <CardDescription>Amounts, currencies, and exchange rate.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <p className="text-sm">
                Send: {agreement.origin_amount} {agreement.origin_currency}
              </p>
              <p className="text-sm">
                Receive: {agreement.counter_amount} {agreement.counter_currency}
              </p>
              <p className="text-sm">Exchange Rate: {agreement.rate}</p>
              <p className="text-sm">Expiry: {new Date(agreement.expiry_date * 1000).toLocaleString()}</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Actions</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              {isProposed ? (
                <>
                  <Button onClick={() => setConfirmAction("accept")}>Accept</Button>
                  <Button variant="destructive" onClick={() => setConfirmAction("reject")}>
                    Reject
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmAction("cancel")}>
                    Cancel
                  </Button>
                </>
              ) : null}
              {isAccepted ? (
                <>
                  <Button onClick={() => navigate("/htlc/new", { state: { agreementId: tradeId, preferredMode: "lock" } })}>
                    Initiate PvP Transfer
                  </Button>
                  <Button variant="outline" onClick={() => navigate("/htlc/new", { state: { agreementId: tradeId, preferredMode: "lockWithHash" } })}>
                    Continue PvP Transfer
                  </Button>
                </>
              ) : null}
              {!isProposed && !isAccepted ? (
                <p className="text-sm text-muted-foreground">No actions available for this agreement state.</p>
              ) : null}
            </CardContent>
          </Card>

          {confirmAction ? (
            <Card>
              <CardHeader>
                <CardTitle>Confirm {confirmAction.charAt(0).toUpperCase() + confirmAction.slice(1)}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <p className="text-sm">
                  Are you sure you want to <span className="font-medium">{confirmAction}</span> this trade agreement? This action cannot be undone.
                </p>
                <div className="flex gap-2">
                  <Button
                    variant={confirmAction === "reject" || confirmAction === "cancel" ? "destructive" : "default"}
                    onClick={() => void onAction(confirmAction)}
                    disabled={storeStatus === "loading"}
                  >
                    {storeStatus === "loading" ? "Processing..." : `Confirm ${confirmAction.charAt(0).toUpperCase() + confirmAction.slice(1)}`}
                  </Button>
                  <Button variant="outline" onClick={() => setConfirmAction(null)}>
                    Go Back
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
