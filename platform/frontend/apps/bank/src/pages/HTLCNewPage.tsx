import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  toast,
} from "@cbweb3/ui";
import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useHtlcStore } from "../stores";

type DurationOption = "1h" | "6h" | "24h" | "custom";

const nowUnix = () => Math.floor(Date.now() / 1000);

export function HTLCNewPage() {
  const navigate = useNavigate();
  const lock = useHtlcStore((state) => state.lock);
  const status = useHtlcStore((state) => state.status);

  const [agreementId, setAgreementId] = useState("");
  const [receiver, setReceiver] = useState("");
  const [amount, setAmount] = useState("");
  const [duration, setDuration] = useState<DurationOption>("1h");
  const [customDateTime, setCustomDateTime] = useState("");
  const [showConfirm, setShowConfirm] = useState(false);

  const timeLock = useMemo(() => {
    const now = nowUnix();
    if (duration === "1h") return now + 3600;
    if (duration === "6h") return now + 21600;
    if (duration === "24h") return now + 86400;
    if (!customDateTime) return now;
    return Math.floor(new Date(customDateTime).getTime() / 1000);
  }, [customDateTime, duration]);

  const validate = () => {
    if (!/^[a-zA-Z0-9_-]{1,64}$/.test(agreementId)) {
      toast.error("Agreement ID must be alphanumeric and may include - or _.");
      return false;
    }
    if (!receiver.trim()) {
      toast.error("Receiver identity is required.");
      return false;
    }
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return false;
    }
    if (timeLock < nowUnix() + 300) {
      toast.error("Timelock must be at least 5 minutes in the future.");
      return false;
    }
    return true;
  };

  const onPrepareLock = () => {
    if (!validate()) {
      return;
    }
    setShowConfirm(true);
  };

  const onConfirmLock = async () => {
    try {
      const result = await lock({
        agreement_id: agreementId,
        receiver,
        amount,
        time_lock: timeLock,
      });
      toast.success("HTLC lock created successfully.");
      navigate(`/htlc/${result.contract_id}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to create HTLC lock.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">New HTLC Lock</h1>
          <p className="text-sm text-muted-foreground">Create a cross-spoke HTLC lock for FX settlement.</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/htlc">Back to history</Link>
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Lock Parameters</CardTitle>
          <CardDescription>All fields are required. Timelock must be in the future.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="agreement-id">Agreement ID</Label>
            <Input
              id="agreement-id"
              placeholder="FX_CROSS_SPOKE_001"
              value={agreementId}
              onChange={(event) => setAgreementId(event.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="receiver">Receiver (Paladin identity)</Label>
            <Input
              id="receiver"
              placeholder="funded_operator@spoke-b-cb"
              value={receiver}
              onChange={(event) => setReceiver(event.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="amount">Amount (tCeBM)</Label>
            <Input id="amount" type="number" min="1" step="1" value={amount} onChange={(event) => setAmount(event.target.value)} />
          </div>

          <div className="space-y-2">
            <Label>Timelock Duration</Label>
            <Select value={duration} onValueChange={(value) => setDuration(value as DurationOption)}>
              <SelectTrigger>
                <SelectValue placeholder="Select duration" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1h">1 hour</SelectItem>
                <SelectItem value="6h">6 hours</SelectItem>
                <SelectItem value="24h">24 hours</SelectItem>
                <SelectItem value="custom">Custom</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {duration === "custom" ? (
            <div className="space-y-2">
              <Label htmlFor="custom-datetime">Custom expiration date/time</Label>
              <Input
                id="custom-datetime"
                type="datetime-local"
                value={customDateTime}
                onChange={(event) => setCustomDateTime(event.target.value)}
              />
            </div>
          ) : null}

          <p className="text-xs text-muted-foreground">
            Expires at unix timestamp: <span className="font-medium">{timeLock}</span>
          </p>

          <Button onClick={onPrepareLock} disabled={status === "loading"}>
            {status === "loading" ? "Submitting..." : "Review Lock"}
          </Button>
        </CardContent>
      </Card>

      {showConfirm ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm HTLC Lock</CardTitle>
            <CardDescription>This action locks funds until settlement or timeout refund.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm">Agreement ID: {agreementId}</p>
            <p className="text-sm">Receiver: {receiver}</p>
            <p className="text-sm">Amount: {amount} tCeBM</p>
            <p className="text-sm">Timelock: {timeLock}</p>
            <div className="flex gap-2">
              <Button onClick={() => void onConfirmLock()} disabled={status === "loading"}>
                {status === "loading" ? "Creating lock..." : "Confirm Lock"}
              </Button>
              <Button variant="outline" onClick={() => setShowConfirm(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
