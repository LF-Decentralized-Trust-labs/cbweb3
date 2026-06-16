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
  const limits = useTransferLimitsStore((state) => state.limits);
  const status = useTransferLimitsStore((state) => state.status);
  const error = useTransferLimitsStore((state) => state.error);
  const fetchLimits = useTransferLimitsStore((state) => state.fetch);
  const createLimit = useTransferLimitsStore((state) => state.create);
  const removeLimit = useTransferLimitsStore((state) => state.remove);

  const [participantId, setParticipantId] = useState("");
  const [currency, setCurrency] = useState("");
  const [maxAmount, setMaxAmount] = useState("");
  const [deleteTargetId, setDeleteTargetId] = useState<string | null>(null);

  useEffect(() => {
    void fetchLimits();
  }, [fetchLimits]);

  const onCreateLimit = async () => {
    if (!maxAmount.trim()) {
      toast.error("Max amount is required.");
      return;
    }
    try {
      await createLimit({
        participant_id: participantId.trim() || undefined,
        currency: currency.trim() || undefined,
        max_amount: maxAmount.trim(),
      });
      toast.success("Transfer limit created.");
      setParticipantId("");
      setCurrency("");
      setMaxAmount("");
    } catch (createError) {
      toast.error(createError instanceof Error ? createError.message : "Unable to create transfer limit");
    }
  };

  const onDeleteLimit = async () => {
    if (!deleteTargetId) return;
    try {
      await removeLimit(deleteTargetId);
      toast.success("Transfer limit removed.");
      setDeleteTargetId(null);
    } catch (deleteError) {
      toast.error(deleteError instanceof Error ? deleteError.message : "Unable to remove transfer limit");
    }
  };

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Limits</CardDescription>
            <CardTitle>{limits.filter((l) => l.is_active).length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Limits</CardDescription>
            <CardTitle>{limits.length}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Create Transfer Limit</CardTitle>
          <CardDescription>
            Set a daily transfer limit. Leave Participant ID blank to apply to all participants, leave Currency blank to
            apply to all currencies.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 md:grid-cols-3">
            <div className="space-y-2">
              <Label htmlFor="participant-id">Participant ID (optional)</Label>
              <Input
                id="participant-id"
                placeholder="bank-a"
                value={participantId}
                onChange={(e) => setParticipantId(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="currency">Currency (optional)</Label>
              <Input
                id="currency"
                placeholder="BRL"
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="max-amount">Daily Max Amount</Label>
              <Input
                id="max-amount"
                placeholder="500000"
                value={maxAmount}
                onChange={(e) => setMaxAmount(e.target.value)}
              />
            </div>
          </div>
          <Button onClick={() => void onCreateLimit()} disabled={status === "loading"}>
            {status === "loading" ? "Creating..." : "Create Limit"}
          </Button>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Configured Transfer Limits</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Participant</TableHead>
                  <TableHead>Currency</TableHead>
                  <TableHead>Daily Max</TableHead>
                  <TableHead>Created At</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {limits.map((limit) => (
                  <TableRow key={limit.limit_id}>
                    <TableCell className="font-mono">{limit.participant_id || <span className="text-muted-foreground">all participants</span>}</TableCell>
                    <TableCell className="font-mono">{limit.currency || <span className="text-muted-foreground">all currencies</span>}</TableCell>
                    <TableCell>{limit.max_amount}</TableCell>
                    <TableCell>{new Date(limit.created_at).toLocaleString()}</TableCell>
                    <TableCell className="text-right">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setDeleteTargetId(limit.limit_id)}
                        disabled={status === "loading"}
                      >
                        Remove
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {!limits.length ? <p className="pt-3 text-sm text-muted-foreground">No transfer limits configured.</p> : null}
          </div>
        </CardContent>
      </Card>

      {deleteTargetId ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Removal</CardTitle>
            <CardDescription>
              Remove limit{" "}
              <span className="font-mono">{deleteTargetId}</span>? This is irreversible.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button variant="destructive" onClick={() => void onDeleteLimit()} disabled={status === "loading"}>
              {status === "loading" ? "Removing..." : "Confirm Remove"}
            </Button>
            <Button variant="outline" onClick={() => setDeleteTargetId(null)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
