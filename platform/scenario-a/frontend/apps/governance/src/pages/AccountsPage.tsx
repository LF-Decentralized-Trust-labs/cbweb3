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

export function AccountsPage() {
  const { accounts, status, error, fetch, freeze } = useAccounts();
  const [search, setSearch] = useState("");
  const [targetAccountId, setTargetAccountId] = useState<string | null>(null);
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

  const onFreeze = async () => {
    if (!targetAccountId) {
      return;
    }
    if (reason.trim().length < 10) {
      toast.error("Reason must contain at least 10 characters.");
      return;
    }
    await freeze({ accountId: targetAccountId, reason });
    toast.success("Account frozen successfully");
    setTargetAccountId(null);
    setReason("");
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>Account Intervention</CardTitle>
          <CardDescription>Freeze participant accounts in emergency scenarios.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input placeholder="Search by account ID or participant" value={search} onChange={(event) => setSearch(event.target.value)} />
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
                    <Badge variant={account.frozen ? "destructive" : "default"}>{account.frozen ? "FROZEN" : "ACTIVE"}</Badge>
                  </TableCell>
                  <TableCell>{account.frozenAt ? new Date(account.frozenAt).toLocaleString() : "—"}</TableCell>
                  <TableCell>{account.frozenReason ?? "—"}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={account.frozen}
                      onClick={() => {
                        setTargetAccountId(account.id);
                        setReason("");
                      }}
                    >
                      Freeze Account
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {targetAccountId ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm Freeze</CardTitle>
            <CardDescription>Account: {targetAccountId}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label>Reason (required)</Label>
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button variant="destructive" onClick={() => void onFreeze()} disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Confirm Freeze"}
              </Button>
              <Button variant="outline" onClick={() => setTargetAccountId(null)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
