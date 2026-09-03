// SPDX-License-Identifier: Apache-2.0

import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Checkbox,
  Input,
  Label,
  Separator,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useComplianceStore } from "../stores";

export function ComplianceCenterPage() {
  const { credentials, fetch, attach, status, error } = useComplianceStore();
  const [transactionId, setTransactionId] = useState("tx_example_001");
  const [selected, setSelected] = useState<string[]>([]);

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const toggleCredential = (id: string) => {
    setSelected((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    );
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Credential Library</CardTitle>
          <CardDescription>Attach ZK-compliance credentials to pending operations</CardDescription>
        </CardHeader>
        <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Select</TableHead>
              <TableHead>Credential</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Issuer</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {credentials.map((credential) => (
              <TableRow key={credential.id}>
                <TableCell>
                  <div className="flex items-center space-x-2">
                    <Checkbox
                      id={credential.id}
                      checked={selected.includes(credential.id)}
                      onCheckedChange={() => toggleCredential(credential.id)}
                    />
                    <Label htmlFor={credential.id} className="text-xs text-muted-foreground">
                      Select
                    </Label>
                  </div>
                </TableCell>
                <TableCell>{credential.id}</TableCell>
                <TableCell>
                  <Badge variant="secondary">{credential.type}</Badge>
                </TableCell>
                <TableCell>{credential.issuer}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Attach Credentials</CardTitle>
          <CardDescription>Structural compliance workflow</CardDescription>
        </CardHeader>
        <CardContent>
        <div className="space-y-2">
          <Label htmlFor="transaction-id">Transaction ID</Label>
          <Input value={transactionId} onChange={(event) => setTransactionId(event.target.value)} placeholder="Transaction ID" />
          <Separator />
          <p className="text-xs text-muted-foreground">Selected credentials: {selected.length}</p>
          <Button onClick={() => void attach(transactionId, selected)} disabled={status === "loading" || !selected.length}>
            Attach Selected
          </Button>
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
        </div>
        </CardContent>
      </Card>
    </div>
  );
}
