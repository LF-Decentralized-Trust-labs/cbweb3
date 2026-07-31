// SPDX-License-Identifier: Apache-2.0

import {
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
import { useEffect, useState } from "react";
import { registryApi } from "../services/api";
import { useTransferLimitsStore } from "../stores";

const SELECT_CLASS =
  "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50";

type ParticipantOption = { bankCode: string; name: string; status: string };

function formatAmount(amount: string, currency: string): string {
  const num = parseFloat(amount);
  if (isNaN(num)) return amount;
  const formatted = new Intl.NumberFormat("en-US").format(num);
  return currency ? `${formatted} ${currency}` : formatted;
}

export function TransferLimitsPage() {
  const { limits, sovereignCurrency, status, error, fetch, create, remove } = useTransferLimitsStore();

  const [participantId, setParticipantId] = useState("");
  const [maxAmount, setMaxAmount] = useState("");
  const [participants, setParticipants] = useState<ParticipantOption[]>([]);

  useEffect(() => {
    void fetch();
    registryApi
      .listParticipants()
      .then(setParticipants)
      .catch(() => setParticipants([]));
  }, [fetch]);

  // A CB only sets limits in its own currency; show the human code (e.g. "COP" from
  // "tCeBM_COP") read-only. The backend defaults the currency to the sovereign one.
  const currencyLabel = sovereignCurrency ? sovereignCurrency.split("_").pop() || sovereignCurrency : "—";

  const onSubmit = async () => {
    if (!maxAmount.trim()) {
      toast.error("Max amount is required");
      return;
    }
    try {
      await create({
        participant_id: participantId.trim() || undefined,
        max_amount: maxAmount.trim(),
      });
      toast.success("Transfer limit created");
      setParticipantId("");
      setMaxAmount("");
    } catch {
      // error is set in store
    }
  };

  const onRemove = async (limitId: string) => {
    await remove(limitId);
    toast.success("Transfer limit removed");
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>New Transfer Limit</CardTitle>
          <CardDescription>
            Set a daily transfer limit for a participant. Leave participant blank to apply broadly. Limits always
            apply in your spoke&apos;s sovereign currency.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="participant">Participant</Label>
            <select
              id="participant"
              className={SELECT_CLASS}
              value={participantId}
              onChange={(e) => setParticipantId(e.target.value)}
            >
              <option value="">All participants</option>
              {participants.map((p) => (
                <option key={p.bankCode} value={p.bankCode}>
                  {p.name} ({p.bankCode})
                </option>
              ))}
            </select>
            <p className="text-xs text-muted-foreground">Registered banks on your spoke.</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="currency">Currency</Label>
            <Input id="currency" value={currencyLabel} readOnly title={sovereignCurrency || undefined} />
            <p className="text-xs text-muted-foreground">Your spoke&apos;s sovereign currency (fixed).</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="max-amount">Daily Max Amount</Label>
            <Input
              id="max-amount"
              value={maxAmount}
              onChange={(e) => setMaxAmount(e.target.value)}
              placeholder="e.g. 1000000"
            />
          </div>
          <div className="md:col-span-3">
            <Button onClick={() => void onSubmit()} disabled={status === "loading"}>
              Create limit
            </Button>
          </div>
          {error ? <p className="md:col-span-3 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Active Limits</CardTitle>
          <CardDescription>Daily transfer limits configured for your spoke participants.</CardDescription>
        </CardHeader>
        <CardContent>
          {limits.length === 0 ? (
            <p className="text-sm text-muted-foreground">No limits configured.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Participant</TableHead>
                  <TableHead>Currency</TableHead>
                  <TableHead>Daily Max</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {limits.map((limit) => (
                  <TableRow key={limit.limit_id}>
                    <TableCell>{limit.participant_id || <span className="text-muted-foreground">all</span>}</TableCell>
                    <TableCell>{limit.currency || <span className="text-muted-foreground">all</span>}</TableCell>
                    <TableCell className="font-mono">{formatAmount(limit.max_amount, limit.currency)}</TableCell>
                    <TableCell className="text-muted-foreground text-sm">
                      {new Date(limit.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => void onRemove(limit.limit_id)}
                        disabled={status === "loading"}
                      >
                        Remove
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
