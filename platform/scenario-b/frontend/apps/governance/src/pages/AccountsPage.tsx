// SPDX-License-Identifier: Apache-2.0

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
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { AccountEntry } from "../types";
import { useAccounts } from "../hooks";

type PendingAction = { accountId: string; mode: "freeze" | "unfreeze" };

const PAGE_SIZE = 10;

// The Central Bank is the governance admin: it can never freeze/unfreeze itself.
const isCentralBank = (account: AccountEntry) => (account.role ?? "").toUpperCase().includes("CENTRAL_BANK");

export function AccountsPage() {
  const { accounts, status, error, fetch, freeze, unfreeze } = useAccounts();
  const [search, setSearch] = useState("");
  const [pending, setPending] = useState<PendingAction | null>(null);
  const [reason, setReason] = useState("");
  const [page, setPage] = useState(1);

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

  const total = filtered.length;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  // Searching resets to the first page.
  useEffect(() => {
    setPage(1);
  }, [search]);

  // Keep the page in range when the list shrinks (e.g. after a status change).
  useEffect(() => {
    setPage((current) => Math.min(current, totalPages));
  }, [totalPages]);

  const paged = useMemo(() => filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE), [filtered, page]);
  const rangeStart = total === 0 ? 0 : (page - 1) * PAGE_SIZE + 1;
  const rangeEnd = Math.min(page * PAGE_SIZE, total);

  const onConfirm = async () => {
    if (!pending) {
      return;
    }
    if (reason.trim().length < 10) {
      toast.error("Reason must contain at least 10 characters.");
      return;
    }
    if (pending.mode === "freeze") {
      await freeze({ accountId: pending.accountId, reason });
      toast.success("Account frozen successfully");
    } else {
      await unfreeze({ accountId: pending.accountId, reason });
      toast.success("Account unfrozen successfully");
    }
    setPending(null);
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
              {paged.map((account) => {
                const admin = isCentralBank(account);
                return (
                  <TableRow key={account.id}>
                    <TableCell className="font-medium">{account.id}</TableCell>
                    <TableCell>
                      {account.participantName}
                      {admin ? (
                        <Badge variant="outline" className="ml-2 align-middle">
                          Admin
                        </Badge>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <Badge variant={account.frozen ? "destructive" : "default"}>{account.frozen ? "FROZEN" : "ACTIVE"}</Badge>
                    </TableCell>
                    <TableCell>{account.frozenAt ? new Date(account.frozenAt).toLocaleString() : "—"}</TableCell>
                    <TableCell>{account.frozenReason ?? "—"}</TableCell>
                    <TableCell className="text-right">
                      {admin ? (
                        <span className="text-xs text-muted-foreground">Central Bank — protected</span>
                      ) : account.frozen ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setPending({ accountId: account.id, mode: "unfreeze" });
                            setReason("");
                          }}
                        >
                          Unfreeze Account
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setPending({ accountId: account.id, mode: "freeze" });
                            setReason("");
                          }}
                        >
                          Freeze Account
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
              {total === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-sm text-muted-foreground">
                    No accounts found.
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>

          {/* Pagination */}
          <div className="flex items-center justify-between gap-2 pt-2">
            <span className="text-xs text-muted-foreground">
              {total === 0 ? "—" : `Showing ${rangeStart}–${rangeEnd} of ${total}`}
            </span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
              >
                <ChevronLeft className="h-4 w-4" />
                Prev
              </Button>
              <span className="text-xs text-muted-foreground">
                Page {page} of {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                disabled={page >= totalPages}
              >
                Next
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {error ? <p className="text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>

      {pending ? (
        <Card>
          <CardHeader>
            <CardTitle>{pending.mode === "freeze" ? "Confirm Freeze" : "Confirm Unfreeze"}</CardTitle>
            <CardDescription>Account: {pending.accountId}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-2">
              <Label>Reason (required)</Label>
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
            </div>
            <div className="flex gap-2">
              <Button
                variant={pending.mode === "freeze" ? "destructive" : "default"}
                onClick={() => void onConfirm()}
                disabled={status === "loading"}
              >
                {status === "loading" ? "Submitting..." : pending.mode === "freeze" ? "Confirm Freeze" : "Confirm Unfreeze"}
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
