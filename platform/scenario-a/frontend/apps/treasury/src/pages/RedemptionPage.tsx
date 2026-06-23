// SPDX-License-Identifier: Apache-2.0

import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Input, Label, Textarea, toast } from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useRedemption, useSupplyAudit } from "../hooks";

const formatAmount = (value: bigint) => Number(value).toLocaleString();

export function RedemptionPage() {
  const { burn, status, error } = useRedemption();
  const { supply, fetch } = useSupplyAudit();
  const [sourceAccount, setSourceAccount] = useState("treasury-reserve-vault-01");
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");

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

  const onSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!amount || !reason.trim()) {
      toast.error("Amount and reason are required");
      return;
    }
    await burn({ sourceAccount, amount, reason });
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
            <Label htmlFor="source">Source account</Label>
            <Input id="source" value={sourceAccount} onChange={(event) => setSourceAccount(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="amount">Amount</Label>
            <Input id="amount" value={amount} onChange={(event) => setAmount(event.target.value)} placeholder="e.g. 50000" />
          </div>
          <div className="space-y-2 md:col-span-2">
            <Label htmlFor="reason">Redemption reason</Label>
            <Textarea id="reason" rows={4} value={reason} onChange={(event) => setReason(event.target.value)} />
          </div>
          <div className="md:col-span-2 rounded-md border border-border bg-muted/40 p-3 text-sm">
            <p>Current supply: {supply ? formatAmount(BigInt(supply.circulatingSupply)) : "-"}</p>
            <p>Projected after burn: {projected !== null ? formatAmount(projected) : "-"}</p>
          </div>
          {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
          <div className="md:col-span-2">
            <Button type="submit" disabled={status === "loading"}>
              Execute burn
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
