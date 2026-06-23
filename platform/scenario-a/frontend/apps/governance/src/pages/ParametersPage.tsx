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
  Textarea,
  toast,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useParameters } from "../hooks";

type EditableForm = {
  txLimitMin: string;
  txLimitMax: string;
  slippageTolerance: string;
  settlementWindowSeconds: string;
};

export function ParametersPage() {
  const { parameters, status, error, fetch, update } = useParameters();
  const [draft, setDraft] = useState<EditableForm | null>(null);
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const form = useMemo<EditableForm>(() => {
    if (draft) {
      return draft;
    }
    return {
      txLimitMin: String(parameters?.txLimitMin ?? ""),
      txLimitMax: String(parameters?.txLimitMax ?? ""),
      slippageTolerance: String(parameters?.slippageTolerance ?? ""),
      settlementWindowSeconds: String(parameters?.settlementWindowSeconds ?? ""),
    };
  }, [draft, parameters]);

  const diffs = useMemo(() => {
    if (!parameters) {
      return [];
    }
    const next = {
      txLimitMin: Number(form.txLimitMin),
      txLimitMax: Number(form.txLimitMax),
      slippageTolerance: Number(form.slippageTolerance),
      settlementWindowSeconds: Number(form.settlementWindowSeconds),
    };
    return (Object.keys(next) as Array<keyof typeof next>)
      .filter((key) => parameters[key] !== next[key])
      .map((key) => ({
        field: key,
        previous: parameters[key],
        proposed: next[key],
      }));
  }, [form, parameters]);

  const onSubmit = async () => {
    if (!parameters) {
      return;
    }
    if (reason.trim().length < 10) {
      toast.error("Reason must contain at least 10 characters.");
      return;
    }
    await update({
      txLimitMin: Number(form.txLimitMin),
      txLimitMax: Number(form.txLimitMax),
      slippageTolerance: Number(form.slippageTolerance),
      settlementWindowSeconds: Number(form.settlementWindowSeconds),
      reason,
    });
    toast.success("Parameters updated");
    setReason("");
    setConfirming(false);
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Global Parameters</CardTitle>
          <CardDescription>Update transaction limits, slippage tolerance, and settlement window.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label>Transaction Minimum</Label>
            <Input
              value={form.txLimitMin}
              onChange={(event) =>
                setDraft((prev) => ({ ...(prev ?? form), txLimitMin: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label>Transaction Maximum</Label>
            <Input
              value={form.txLimitMax}
              onChange={(event) =>
                setDraft((prev) => ({ ...(prev ?? form), txLimitMax: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label>Slippage Tolerance</Label>
            <Input
              value={form.slippageTolerance}
              onChange={(event) =>
                setDraft((prev) => ({ ...(prev ?? form), slippageTolerance: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2">
            <Label>Settlement Window (seconds)</Label>
            <Input
              value={form.settlementWindowSeconds}
              onChange={(event) =>
                setDraft((prev) => ({ ...(prev ?? form), settlementWindowSeconds: event.target.value }))
              }
            />
          </div>
          <div className="space-y-2 md:col-span-2">
            <Label>Reason (required)</Label>
            <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
          </div>

          <div className="md:col-span-2">
            <Button onClick={() => setConfirming(true)}>Review Changes</Button>
          </div>
          {error ? <p className="md:col-span-2 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {confirming ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Parameter Diff</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Field</TableHead>
                  <TableHead>Current</TableHead>
                  <TableHead>Proposed</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {diffs.length ? (
                  diffs.map((diff) => (
                    <TableRow key={diff.field}>
                      <TableCell>{diff.field}</TableCell>
                      <TableCell>{String(diff.previous)}</TableCell>
                      <TableCell>{String(diff.proposed)}</TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={3} className="text-muted-foreground">
                      No changes detected.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>

            <div className="flex gap-2">
              <Button onClick={() => void onSubmit()} disabled={status === "loading" || diffs.length === 0}>
                {status === "loading" ? "Submitting..." : "Confirm Update"}
              </Button>
              <Button variant="outline" onClick={() => setConfirming(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
