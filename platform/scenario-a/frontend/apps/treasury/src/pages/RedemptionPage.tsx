// SPDX-License-Identifier: Apache-2.0

import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, ConfirmActionDialog, Input, Label, Textarea, toast } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useRedemption, useSupplyAudit } from "../hooks";
import { formatTokenAmount } from "../lib/format";

/**
 * The identity that actually signs the burn.
 *
 * This screen used to offer an editable "Source account" field, pre-filled with
 * a plausible-looking vault name. The value was discarded: the Zeto withdraw is
 * signed by the gateway's own configured Paladin identity, and the orchestrator
 * takes the `from` argument as informational only. A field that suggests a
 * choice which does not exist is worse than no field, so the value is now fixed
 * and shown for what it is. It still travels to the server, where it is
 * recorded in the audit trail alongside the operator who ordered the burn.
 */
const GATEWAY_SIGNING_IDENTITY = "gateway-paladin-identity";

export function RedemptionPage() {
  const { burn, status, error } = useRedemption();
  const { supply, fetch } = useSupplyAudit();
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const projected = useMemo(() => {
    if (!supply || !amount) {
      return null;
    }
    const current = BigInt(supply.circulatingSupply);
    const burnAmount = BigInt(amount || "0");
    return current >= burnAmount ? current - burnAmount : 0n;
  }, [supply, amount]);

  const exceedsBalance = useMemo(() => {
    if (!supply || !amount) {
      return false;
    }
    try {
      return BigInt(amount) > BigInt(supply.circulatingSupply);
    } catch {
      return false;
    }
  }, [supply, amount]);

  const onSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!amount || !reason.trim()) {
      toast.error("Amount and reason are required");
      return;
    }
    setConfirming(true);
  };

  const onConfirmBurn = async () => {
    setConfirming(false);
    // Report what actually happened. This used to toast success unconditionally, so a
    // rejected burn — an insufficient-funds 500 from the orchestrator, say — told the
    // operator it had gone through, and cleared the form so there was nothing left to
    // show what had been attempted.
    const failure = await burn({ sourceAccount: GATEWAY_SIGNING_IDENTITY, amount, reason });
    if (failure) {
      toast.error(failure);
      return;
    }
    toast.success("Burn operation submitted");
    setAmount("");
    setReason("");
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Redemption (Burn)</CardTitle>
        <CardDescription>Burn tokens to contract circulating supply and simulate fiat off-ramp.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="grid gap-4 md:grid-cols-2" onSubmit={onSubmit}>
          <div className="space-y-2">
            <Label htmlFor="source">Signing identity</Label>
            <Input id="source" value={GATEWAY_SIGNING_IDENTITY} readOnly disabled />
            <p className="text-xs text-muted-foreground">The burn is signed by this gateway&apos;s configured Paladin identity.</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="amount">Amount</Label>
            <Input id="amount" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="e.g. 50000" />
          </div>
          <div className="space-y-2 md:col-span-2">
            <Label htmlFor="reason">Redemption reason</Label>
            <Textarea id="reason" rows={4} value={reason} onChange={(event) => setReason(event.target.value)} />
            <p className="text-xs text-muted-foreground">Required. Recorded in the audit trail as the justification for this burn.</p>
          </div>
          <div className="md:col-span-2 rounded-md border border-border bg-muted/40 p-3 text-sm">
            {/* This reads GET /token/balance — the balance of this gateway's own
                account, not the network-wide circulating supply. It used to be
                labelled "Current supply", which overstated what the figure is
                (finding R2-M-8). */}
            <p>Balance of the signing account: {supply ? formatTokenAmount(BigInt(supply.circulatingSupply)) : "-"}</p>
            <p>Projected after burn: {projected !== null ? formatTokenAmount(projected) : "-"}</p>
            {exceedsBalance ? (
              <p className="mt-1 text-destructive">The amount exceeds the account balance; the projection above is floored at zero.</p>
            ) : null}
          </div>
          {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
          <div className="md:col-span-2">
            <Button type="submit" disabled={status === "loading"}>
              Execute burn
            </Button>
          </div>
        </form>
      </CardContent>
      <ConfirmActionDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Confirm burn"
        description="This destroys tokens on-chain and cannot be undone; reversing it requires a separately authorised issuance."
        fields={[
          { label: "Signing identity", value: GATEWAY_SIGNING_IDENTITY },
          { label: "Amount", value: amount },
          {
            // The projection is floored at zero, so an over-sized burn would
            // otherwise read as a plausible "0" here — on the very step meant to
            // catch that mistake. Say so instead.
            label: "Balance after",
            value: exceedsBalance
              ? "exceeds the account balance"
              : projected !== null
                ? formatTokenAmount(projected)
                : "-",
          },
          { label: "Reason", value: reason.trim() },
        ]}
        confirmLabel="Confirm burn"
        destructive
        busy={status === "loading"}
        onConfirm={() => void onConfirmBurn()}
      />
    </Card>
  );
}
