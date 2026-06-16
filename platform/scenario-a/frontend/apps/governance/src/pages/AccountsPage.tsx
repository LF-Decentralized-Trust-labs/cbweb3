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
import { useAccounts } from "../hooks";

type PendingAction = { accountId: string; action: "freeze" | "unfreeze" };

export function AccountsPage() {
  const { accounts, status, error, fetch, freeze, unfreeze } = useAccounts();
  const [search, setSearch] = useState("");
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const filtered = useMemo(() => {
    const term = search.toLowerCase().trim();
    if (!term) return accounts;
    return accounts.filter(
      (account) => account.id.toLowerCase().includes(term) || account.participantName.toLowerCase().includes(term),
    );
  }, [accounts, search]);

  const onConfirm = async () => {
    if (!pending) return;
    if (reason.trim().length < 10) {
      toast.error("Reason must contain at least 10 characters.");
      return;
    }
    if (pending.action === "freeze") {
      await freeze({ accountId: pending.accountId, reason });
      toast.success("Account frozen successfully.");
    } else {
      await unfreeze({ accountId: pending.accountId, reason });
      toast.success("Account unfrozen successfully.");
    }
    setPending(null);
    setReason("");
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Account Intervention</CardTitle>
          <CardDescription>Freeze or unfreeze participant accounts.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input
            placeholder="Search by account ID or participant"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Account</TableHead>
                <TableHead>Participant</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Frozen At</TableHead>
                <TableHead>Reason</TableHead>
                <TableHead className="text-right">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((account) => (
                <TableRow key={account.id}>
                  <TableCell className="font-medium">{account.id}</TableCell>
                  <TableCell>{account.participantName}</TableCell>
                  <TableCell>
                    <Badge variant={account.frozen ? "destructive" : "default"}>
                      {account.frozen ? "FROZEN" : "ACTIVE"}
                    </Badge>
                  </TableCell>
                  <TableCell>{account.frozenAt ? new Date(account.frozenAt).toLocaleString() : "—"}</TableCell>
                  <TableCell>{account.frozenReason ?? "—"}</TableCell>
                  <TableCell className="text-right">
                    {account.frozen ? (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => { setPending({ accountId: account.id, action: "unfreeze" }); setReason(""); }}
                      >
                        Unfreeze
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => { setPending({ accountId: account.id, action: "freeze" }); setReason(""); }}
                      >
                        Freeze
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {pending ? (
        <Card>
          <CardHeader>
            <CardTitle>{pending.action === "freeze" ? "Confirm Freeze" : "Confirm Unfreeze"}</CardTitle>
            <CardDescription>Account: {pending.accountId}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label>Reason (required)</Label>
              <Textarea value={reason} onChange={(e) => setReason(e.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button
                variant={pending.action === "freeze" ? "destructive" : "default"}
                onClick={() => void onConfirm()}
                disabled={status === "loading"}
              >
                {status === "loading" ? "Submitting..." : pending.action === "freeze" ? "Confirm Freeze" : "Confirm Unfreeze"}
              </Button>
              <Button variant="outline" onClick={() => setPending(null)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
