import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  DatePicker,
  Input,
  Label,
  Progress,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
} from "@cbweb3/ui";
import { useEffect, useState } from "react";
import { useHtlcStore } from "../stores";

export function HTLCTradingPage() {
  const agreements = useHtlcStore((state) => state.agreements);
  const locks = useHtlcStore((state) => state.locks);
  const fetch = useHtlcStore((state) => state.fetch);
  const createAgreement = useHtlcStore((state) => state.createAgreement);
  const lockFunds = useHtlcStore((state) => state.lockFunds);
  const settle = useHtlcStore((state) => state.settle);
  const status = useHtlcStore((state) => state.status);
  const error = useHtlcStore((state) => state.error);

  const [step, setStep] = useState(1);
  const [counterparty, setCounterparty] = useState("Bank-US");
  const [rate, setRate] = useState("5.10");
  const [notional, setNotional] = useState("10000");
  const [expiryAt, setExpiryAt] = useState<Date | undefined>(
    new Date(Date.now() + 60 * 60 * 1000),
  );
  const [agreementIdForLock, setAgreementIdForLock] = useState<string>("");
  const [secret, setSecret] = useState("my-secret");
  const [lastHashLock, setLastHashLock] = useState("");
  const [lockIdForSettle, setLockIdForSettle] = useState<string>("");

  useEffect(() => {
    void fetch();
  }, [fetch]);

  const stepProgress = (step / 3) * 100;

  const generateSecret = () => {
    const bytes = new Uint8Array(16);
    window.crypto.getRandomValues(bytes);
    const generated = Array.from(bytes)
      .map((value) => value.toString(16).padStart(2, "0"))
      .join("");
    setSecret(generated);
  };

  const onCreateAgreement = async () => {
    const agreementId = await createAgreement({
      counterparty,
      rate,
      notional,
      expiryAt: (expiryAt ?? new Date()).toISOString(),
    });
    if (agreementId) {
      setAgreementIdForLock(agreementId);
      setStep(2);
    }
  };

  const onLock = async () => {
    const hashLock = await lockFunds(agreementIdForLock, secret);
    if (hashLock) {
      setLastHashLock(hashLock);
      const newestLock = useHtlcStore.getState().locks[0];
      if (newestLock) setLockIdForSettle(newestLock.id);
      setStep(3);
    }
  };

  const onSettle = async () => {
    await settle(lockIdForSettle, secret);
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>HTLC Wizard</CardTitle>
          <CardDescription>
            4-corner flow: Agreement → Lock → Settle
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4 space-y-2">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">Step {step} of 3</p>
              <Badge variant="secondary">Scenario A</Badge>
            </div>
            <Progress value={stepProgress} />
          </div>

          <Tabs
            value={String(step)}
            onValueChange={(value) => setStep(Number(value))}
          >
            <TabsList>
              <TabsTrigger value="1">1. Agreement</TabsTrigger>
              <TabsTrigger value="2">2. Lock</TabsTrigger>
              <TabsTrigger value="3">3. Settle</TabsTrigger>
            </TabsList>

            <TabsContent value="1" className="space-y-2">
              <Label>Counterparty bank</Label>
              <Input
                value={counterparty}
                onChange={(event) => setCounterparty(event.target.value)}
                placeholder="Counterparty bank"
              />
              <Label>FX rate</Label>
              <Input
                value={rate}
                onChange={(event) => setRate(event.target.value)}
                placeholder="FX rate"
              />
              <Label>Notional amount</Label>
              <Input
                value={notional}
                onChange={(event) => setNotional(event.target.value)}
                placeholder="Notional amount"
              />
              <Label>Expiry</Label>
              <DatePicker
                value={expiryAt}
                onChange={setExpiryAt}
                placeholder="Select expiry date"
              />
              <Button
                onClick={() => void onCreateAgreement()}
                disabled={status === "loading"}
              >
                Create Agreement
              </Button>
            </TabsContent>

            <TabsContent value="2" className="space-y-2">
              <Label>Agreement</Label>
              <Select
                value={agreementIdForLock}
                onValueChange={setAgreementIdForLock}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select agreement" />
                </SelectTrigger>
                <SelectContent>
                  {agreements.map((agreement) => (
                    <SelectItem key={agreement.id} value={agreement.id}>
                      {agreement.id} · {agreement.counterparty} ·{" "}
                      {agreement.notional}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Label>Secret</Label>
              <Textarea
                value={secret}
                onChange={(event) => setSecret(event.target.value)}
                placeholder="Secret used to generate SHA-256 hashlock"
              />
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={generateSecret}
                >
                  Generate Secret
                </Button>
                <Button
                  onClick={() => void onLock()}
                  disabled={
                    status === "loading" || !agreementIdForLock || !secret
                  }
                >
                  Lock Funds
                </Button>
              </div>
              {lastHashLock ? (
                <p className="text-xs text-muted-foreground">
                  HashLock: {lastHashLock}
                </p>
              ) : null}
            </TabsContent>

            <TabsContent value="3" className="space-y-2">
              <Label>Lock</Label>
              <Select
                value={lockIdForSettle}
                onValueChange={setLockIdForSettle}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select lock" />
                </SelectTrigger>
                <SelectContent>
                  {locks.map((lock) => (
                    <SelectItem key={lock.id} value={lock.id}>
                      {lock.id} · {lock.amount} · {lock.status}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Label>Secret</Label>
              <Textarea
                value={secret}
                onChange={(event) => setSecret(event.target.value)}
                placeholder="Secret to settle hashlock"
              />
              <Button
                variant="outline"
                onClick={() => void onSettle()}
                disabled={status === "loading" || !lockIdForSettle}
              >
                Settle Lock
              </Button>
            </TabsContent>
          </Tabs>

          {error ? <p className="pt-2 text-sm text-red-600">{error}</p> : null}
        </CardContent>
      </Card>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Agreements</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Counterparty</TableHead>
                <TableHead>Rate</TableHead>
                <TableHead>Notional</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agreements.map((agreement) => (
                <TableRow key={agreement.id}>
                  <TableCell>{agreement.id}</TableCell>
                  <TableCell>{agreement.counterparty}</TableCell>
                  <TableCell>{agreement.rate}</TableCell>
                  <TableCell>{agreement.notional}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!agreements.length ? (
            <p className="pt-2 text-sm text-muted-foreground">
              No agreements yet.
            </p>
          ) : null}
        </CardContent>
      </Card>

      <Card className="lg:col-span-2">
        <CardHeader>
          <CardTitle>Locks</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>Agreement</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {locks.map((lock) => (
                <TableRow key={lock.id}>
                  <TableCell>{lock.id}</TableCell>
                  <TableCell>{lock.agreementId}</TableCell>
                  <TableCell>{lock.amount}</TableCell>
                  <TableCell>
                    <Badge
                      variant={
                        lock.status === "SETTLED" ? "success" : "secondary"
                      }
                    >
                      {lock.status}
                    </Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {!locks.length ? (
            <p className="pt-2 text-sm text-muted-foreground">No locks yet.</p>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
