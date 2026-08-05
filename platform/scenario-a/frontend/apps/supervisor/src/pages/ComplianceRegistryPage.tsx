// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useMemo, useState } from "react";
import { useParticipantsStore } from "../stores";

export function ComplianceRegistryPage() {
  const { participants, status, error, fetch } = useParticipantsStore();
  const [query, setQuery] = useState("");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const filteredParticipants = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return participants;

    return participants.filter((participant) => {
      return (
        participant.institutionName.toLowerCase().includes(normalizedQuery) ||
        participant.evmAddress.toLowerCase().includes(normalizedQuery) ||
        participant.jurisdictionCode.toLowerCase().includes(normalizedQuery)
      );
    });
  }, [participants, query]);

  const activeCount = useMemo(
    () => participants.filter((participant) => participant.credentialStatus === "ACTIVE").length,
    [participants],
  );

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Participants</CardDescription>
            <CardTitle>{participants.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Active Credentials</CardDescription>
            <CardTitle>{activeCount}</CardTitle>
          </CardHeader>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>Compliance Registry</CardTitle>
          <CardDescription>Read-only participant and credential visibility for supervisory oversight.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search by institution, address, or jurisdiction"
          />

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Institution</TableHead>
                <TableHead>EVM Address</TableHead>
                <TableHead>Jurisdiction</TableHead>
                <TableHead>Credential</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredParticipants.map((participant) => (
                <TableRow key={participant.id}>
                  <TableCell>{participant.institutionName}</TableCell>
                  <TableCell className="font-mono text-xs">{participant.evmAddress}</TableCell>
                  <TableCell>{participant.jurisdictionCode}</TableCell>
                  <TableCell>
                    <Badge variant={participant.credentialStatus === "ACTIVE" ? "success" : "destructive"}>
                      {participant.credentialStatus}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {status === "loading" ? <p className="text-sm text-muted-foreground">Loading registry...</p> : null}
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          {!filteredParticipants.length && status !== "loading" ? (
            <p className="text-sm text-muted-foreground">No participants match the current filter.</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
