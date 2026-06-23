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
import { useTransferLimitsStore } from "../stores";

export function TransferLimitsPage() {
  const { limits, status, error, fetch, create, remove } = useTransferLimitsStore();

  const [participantId, setParticipantId] = useState("");
  const [currency, setCurrency] = useState("");
  const [maxAmount, setMaxAmount] = useState("");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const onSubmit = async () => {
    if (!maxAmount.trim()) {
      toast.error("Max amount is required");
      return;
    }
    try {
      await create({
        participant_id: participantId.trim() || undefined,
        currency: currency.trim() || undefined,
        max_amount: maxAmount.trim(),
      });
      toast.success("Transfer limit created");
      setParticipantId("");
      setCurrency("");
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
            Set a daily transfer limit for a participant and/or currency. Leave a field blank to apply the limit
            broadly.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-3">
          <div className="space-y-2">
            <Label htmlFor="participant">Participant ID</Label>
            <Input
              id="participant"
              value={participantId}
              onChange={(e) => setParticipantId(e.target.value)}
              placeholder="bank-a (leave blank for all)"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="currency">Currency</Label>
            <Input
              id="currency"
              value={currency}
              onChange={(e) => setCurrency(e.target.value)}
              placeholder="BRL (leave blank for all)"
            />
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
                  <TableHead>Daily Max (human)</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {limits.map((limit) => (
                  <TableRow key={limit.limit_id}>
                    <TableCell>{limit.participant_id || <span className="text-muted-foreground">all</span>}</TableCell>
                    <TableCell>{limit.currency || <span className="text-muted-foreground">all</span>}</TableCell>
                    <TableCell className="font-mono">{limit.max_amount}</TableCell>
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
