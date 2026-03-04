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
  Separator,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Textarea,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useTokenStore } from "../stores";

const statusVariant = (status: string): "warning" | "default" | "success" | "destructive" | "outline" => {
  const variants: Record<string, "warning" | "default" | "success" | "destructive"> = {
    PENDING: "warning",
    LOCKED: "default",
    SETTLED: "success",
    FAILED: "destructive",
    CONFIRMED: "success",
  };
  return variants[status] ?? "outline";
};

export function TokenManagementPage() {
  const { balance, transactions, fetch, mint, transfer, status, error } = useTokenStore();
  const [mintAmount, setMintAmount] = useState("1000");
  const [mintFiatProof, setMintFiatProof] = useState("proof-ref-001");
  const [toAddress, setToAddress] = useState("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed");
  const [transferAmount, setTransferAmount] = useState("250");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  return (
    <div className="space-y-4">
      <section className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Public Balance</CardDescription>
            <CardTitle>{balance?.publicBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="secondary">Available</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Private Balance</CardDescription>
            <CardTitle>{balance?.privateBalance ?? "-"} tCeBM</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant="outline">View-key protected</Badge>
          </CardContent>
        </Card>
      </section>

      <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Mint tCeBM</CardTitle>
          <CardDescription>Burn-to-mint structure (mock)</CardDescription>
        </CardHeader>
        <CardContent>
        <div className="space-y-2">
          <Label htmlFor="mint-amount">Amount</Label>
          <Input value={mintAmount} onChange={(event) => setMintAmount(event.target.value)} placeholder="Amount" />
          <Label htmlFor="mint-proof">Fiat proof reference</Label>
          <Textarea value={mintFiatProof} onChange={(event) => setMintFiatProof(event.target.value)} placeholder="Fiat proof reference" />
          <Separator />
          <Button onClick={() => void mint({ amount: mintAmount, fiatProofRef: mintFiatProof })} disabled={status === "loading"}>
            Mint
          </Button>
        </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Transfer tCeBM</CardTitle>
          <CardDescription>Shielded transfer is represented structurally</CardDescription>
        </CardHeader>
        <CardContent>
        <div className="space-y-2">
          <Label htmlFor="transfer-to">Recipient address</Label>
          <Input value={toAddress} onChange={(event) => setToAddress(event.target.value)} placeholder="Recipient address" />
          <Label htmlFor="transfer-amount">Amount</Label>
          <Input value={transferAmount} onChange={(event) => setTransferAmount(event.target.value)} placeholder="Amount" />
          <div className="flex items-center gap-2">
            <Badge variant="outline">Shielded</Badge>
            <Badge variant="secondary">ZK Ready</Badge>
          </div>
          <Button variant="outline" onClick={() => void transfer({ toAddress, amount: transferAmount, shielded: true })} disabled={status === "loading"}>
            Transfer (Shielded)
          </Button>
        </div>
        </CardContent>
      </Card>
      </div>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Balances</CardTitle>
        </CardHeader>
        <CardContent>
        <p className="text-sm">Public: {balance?.publicBalance ?? "-"}</p>
        <p className="text-sm">Private: {balance?.privateBalance ?? "-"}</p>
        {error ? <p className="text-sm text-red-600">{error}</p> : null}
        </CardContent>
      </Card>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Token Transactions</CardTitle>
        </CardHeader>
        <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Type</TableHead>
              <TableHead>Amount</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {transactions.map((tx) => (
              <TableRow key={tx.id}>
                <TableCell>{tx.kind}</TableCell>
                <TableCell>{tx.amount}</TableCell>
                <TableCell>
                  <Badge variant={statusVariant(tx.status)}>{tx.status}</Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        </CardContent>
      </Card>
    </div>
  );
}
