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
import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { BalanceWidget } from "../components/common/BalanceWidget";
import { useFxAgreementStore } from "../stores/fx-agreement.store";
import { useHtlcStore, usePaymentStore } from "../stores";

type DurationOption = "none" | "1h" | "6h" | "24h" | "custom";
type FormMode = "lock" | "lockWithHash";
type RoleOption = "originator" | "counterparty";

const nowUnix = () => Math.floor(Date.now() / 1000);

export function HTLCNewPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const routeState = location.state as { agreementId?: string; preferredMode?: FormMode } | null;

  const lock = useHtlcStore((state) => state.lock);
  const lockWithHash = useHtlcStore((state) => state.lockWithHash);
  const status = useHtlcStore((state) => state.status);
  const fetchPayments = usePaymentStore((state) => state.fetchAll);
  const balance = usePaymentStore((state) => state.balance);
  const paymentStatus = usePaymentStore((state) => state.status);
  const tCeBMDecimals = usePaymentStore((state) => state.tCeBMDecimals);

  const fetchAgreements = useFxAgreementStore((s) => s.fetchAll);
  const agreements = useFxAgreementStore((s) => s.agreements);

  const [mode, setMode] = useState<FormMode>(routeState?.preferredMode ?? "lock");

  // Agreement picker
  const [selectedAgreementId, setSelectedAgreementId] = useState<string>(routeState?.agreementId ?? "");
  const [selectedRole, setSelectedRole] = useState<RoleOption | null>(
    routeState?.preferredMode === "lock" ? "originator" : routeState?.preferredMode === "lockWithHash" ? "counterparty" : null,
  );
  const [prefillSource, setPrefillSource] = useState<string | null>(routeState?.agreementId ?? null);

  const [agreementId, setAgreementId] = useState("");
  const [lockReceiver, setLockReceiver] = useState("");
  const [lockAmount, setLockAmount] = useState("");
  const [duration, setDuration] = useState<DurationOption>("none");
  const [customDateTime, setCustomDateTime] = useState("");
  const [showLockConfirm, setShowLockConfirm] = useState(false);

  const [hashLock, setHashLock] = useState("");
  const [hashReceiver, setHashReceiver] = useState("");
  const [hashAmount, setHashAmount] = useState("");
  const [hashAgreementId, setHashAgreementId] = useState("");
  const [showLockWithHashConfirm, setShowLockWithHashConfirm] = useState(false);

  useEffect(() => {
    void fetchPayments();
    void fetchAgreements({ state: "FX_STATE_ACCEPTED" });
  }, [fetchPayments, fetchAgreements]);

  // Auto-prefill when navigated from agreement detail page
  useEffect(() => {
    if (routeState?.agreementId && agreements.length > 0) {
      const ag = agreements.find((a) => a.trade_id === routeState.agreementId);
      if (ag && selectedRole) {
        applyPrefill(ag.trade_id, selectedRole);
      }
    }
    // Only run once on mount after agreements load
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agreements]);

  const selectedAgreement = agreements.find((a) => a.trade_id === selectedAgreementId) ?? null;

  function applyPrefill(tradeId: string, role: RoleOption) {
    const ag = agreements.find((a) => a.trade_id === tradeId);
    if (!ag) return;
    if (role === "originator") {
      setMode("lock");
      setAgreementId(tradeId);
      setLockReceiver(ag.spoke_a_receiver ?? "");
      setLockAmount(ag.origin_amount);
      setPrefillSource(tradeId);
    } else {
      setMode("lockWithHash");
      setHashAgreementId(tradeId);
      setHashReceiver(ag.spoke_b_receiver ?? "");
      setHashAmount(ag.counter_amount);
      setPrefillSource(tradeId);
    }
  }

  function onSelectAgreement(tradeId: string) {
    setSelectedAgreementId(tradeId);
    setSelectedRole(null);
    setPrefillSource(null);
  }

  function onSelectRole(role: RoleOption) {
    setSelectedRole(role);
    if (selectedAgreementId) {
      applyPrefill(selectedAgreementId, role);
    }
  }

  function onClearAgreement() {
    setSelectedAgreementId("");
    setSelectedRole(null);
    setPrefillSource(null);
    setAgreementId("");
    setLockReceiver("");
    setLockAmount("");
    setHashAgreementId("");
    setHashReceiver("");
    setHashAmount("");
  }

  const timeLock = useMemo(() => {
    const now = nowUnix();
    if (duration === "none") return undefined;
    if (duration === "1h") return now + 3600;
    if (duration === "6h") return now + 21600;
    if (duration === "24h") return now + 86400;
    if (!customDateTime) return undefined;
    return Math.floor(new Date(customDateTime).getTime() / 1000);
  }, [customDateTime, duration]);

  const validateLock = () => {
    if (!lockReceiver.trim()) {
      toast.error("Receiver identity is required.");
      return false;
    }
    if (!/^\d+$/.test(lockAmount) || Number(lockAmount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return false;
    }
    if (agreementId && !/^[a-zA-Z0-9_-]{1,64}$/.test(agreementId)) {
      toast.error("Agreement ID must be alphanumeric and may include - or _.");
      return false;
    }
    if (timeLock !== undefined && timeLock < nowUnix() + 300) {
      toast.error("Timelock must be at least 5 minutes in the future.");
      return false;
    }
    return true;
  };

  const validateLockWithHash = () => {
    if (!hashReceiver.trim()) {
      toast.error("Receiver identity is required.");
      return false;
    }
    if (!/^\d+$/.test(hashAmount) || Number(hashAmount) <= 0) {
      toast.error("Amount must be a positive integer.");
      return false;
    }
    if (!/^([A-Fa-f0-9]{64}|0x[A-Fa-f0-9]{64})$/.test(hashLock.trim())) {
      toast.error("Settlement Code must be a valid 64-character hex string.");
      return false;
    }
    return true;
  };

  const onPrepareLock = () => {
    if (!validateLock()) return;
    setShowLockConfirm(true);
  };

  const onPrepareLockWithHash = () => {
    if (!validateLockWithHash()) return;
    setShowLockWithHashConfirm(true);
  };

  const onConfirmLock = async () => {
    try {
      const result = await lock({
        receiver: lockReceiver,
        amount: lockAmount,
        ...(agreementId.trim() ? { agreement_id: agreementId.trim() } : {}),
        ...(timeLock !== undefined ? { time_lock: timeLock } : {}),
      });
      toast.success("PvP transfer initiated successfully.");
      navigate(`/htlc/${result.contract_id}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to initiate PvP transfer.");
    }
  };

  const onConfirmLockWithHash = async () => {
    try {
      const result = await lockWithHash({
        hash_lock: hashLock.trim(),
        receiver: hashReceiver,
        amount: hashAmount,
        ...(hashAgreementId.trim() ? { agreement_id: hashAgreementId.trim() } : {}),
      });
      toast.success("PvP transfer continuation submitted successfully.");
      navigate(`/htlc/${result.contract_id}`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Unable to continue PvP transfer.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold">PvP Settlement</h1>
          <p className="text-sm text-muted-foreground">Use one form to initiate a new transfer or continue with an existing settlement reference.</p>
        </div>
        <Button asChild variant="outline">
          <Link to="/htlc">Back to history</Link>
        </Button>
      </div>

      <BalanceWidget balance={balance} loading={paymentStatus === "loading" && balance === null} decimals={tCeBMDecimals} />

      {/* ── FX Agreement Picker ──────────────────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>Link to FX Agreement <span className="text-sm font-normal text-muted-foreground">(optional)</span></CardTitle>
          <CardDescription>Select an accepted FX agreement to pre-fill the form below.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex items-center gap-2">
            <Select value={selectedAgreementId} onValueChange={onSelectAgreement}>
              <SelectTrigger className="flex-1">
                <SelectValue placeholder={agreements.length === 0 ? "No accepted agreements" : "Select an agreement…"} />
              </SelectTrigger>
              <SelectContent>
                {agreements.map((a) => (
                  <SelectItem key={a.trade_id} value={a.trade_id}>
                    {a.trade_id.slice(0, 8)}… — {a.origin_amount} {a.origin_currency} ↔ {a.counter_amount} {a.counter_currency}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {selectedAgreementId ? (
              <Button size="sm" variant="outline" onClick={onClearAgreement}>
                Clear
              </Button>
            ) : null}
          </div>

          {selectedAgreement ? (
            <div className="space-y-2 rounded-md border p-3">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Select your role in this trade</p>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant={selectedRole === "originator" ? "default" : "outline"}
                  onClick={() => onSelectRole("originator")}
                >
                  I am the Originator (Spoke-A)
                </Button>
                <Button
                  size="sm"
                  variant={selectedRole === "counterparty" ? "default" : "outline"}
                  onClick={() => onSelectRole("counterparty")}
                >
                  I am the Counterparty (Spoke-B)
                </Button>
              </div>
              {selectedRole ? (
                <p className="text-xs text-muted-foreground">
                  {selectedRole === "originator"
                    ? `Pre-filling: receiver = ${selectedAgreement.spoke_a_receiver ?? "—"}, amount = ${selectedAgreement.origin_amount} ${selectedAgreement.origin_currency}`
                    : `Pre-filling: receiver = ${selectedAgreement.spoke_b_receiver ?? "—"}, amount = ${selectedAgreement.counter_amount} ${selectedAgreement.counter_currency}`}
                </p>
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* ── Mode selector ───────────────────────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>Choose PvP Action</CardTitle>
          <CardDescription>Each action has its own dedicated form to avoid confusion.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          <Button variant={mode === "lock" ? "default" : "outline"} onClick={() => setMode("lock")}>
            Initiate PvP Transfer
          </Button>
          <Button variant={mode === "lockWithHash" ? "default" : "outline"} onClick={() => setMode("lockWithHash")}>
            Continue PvP Transfer
          </Button>
        </CardContent>
      </Card>

      {mode === "lock" ? (
        <Card>
          <CardHeader>
            <CardTitle>Initiate PvP Transfer</CardTitle>
            <CardDescription>Start the PvP settlement process. Receiver and amount are required.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {prefillSource && mode === "lock" ? (
              <p className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">
                Pre-filled from agreement <span className="font-medium">{prefillSource.slice(0, 8)}…</span>. You may edit any field.
              </p>
            ) : null}

            <div className="space-y-2">
              <Label htmlFor="agreement-id">Agreement ID (optional)</Label>
              <Input
                id="agreement-id"
                placeholder="FX_CROSS_SPOKE_001"
                value={agreementId}
                onChange={(event) => setAgreementId(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="lock-receiver">Receiver (Paladin identity)</Label>
              <Input
                id="lock-receiver"
                placeholder="funded_operator@spoke-a-bank-c"
                value={lockReceiver}
                onChange={(event) => setLockReceiver(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="lock-amount">Amount (tCeBM)</Label>
              <Input
                id="lock-amount"
                type="number"
                min="1"
                step="1"
                value={lockAmount}
                onChange={(event) => setLockAmount(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label>Timelock Duration (optional)</Label>
              <Select value={duration} onValueChange={(value) => setDuration(value as DurationOption)}>
                <SelectTrigger>
                  <SelectValue placeholder="Select duration" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No explicit timelock</SelectItem>
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
              {timeLock !== undefined ? (
                <>
                  Expires at unix timestamp: <span className="font-medium">{timeLock}</span>
                </>
              ) : (
                "No explicit timelock will be sent. Backend defaults apply."
              )}
            </p>

            <Button onClick={onPrepareLock} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Review PvP Transfer"}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Continue PvP Transfer</CardTitle>
            <CardDescription>Continue the flow from the receiver side using an existing settlement reference.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {prefillSource && mode === "lockWithHash" ? (
              <p className="rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">
                Pre-filled from agreement <span className="font-medium">{prefillSource.slice(0, 8)}…</span>. You may edit any field.
              </p>
            ) : null}

            <div className="space-y-2">
              <Label htmlFor="hash-lock">Settlement Code</Label>
              <Input
                id="hash-lock"
                placeholder="c50b6abba36bfcab4508b61b080e2163736bbb145fd1adb4b396273765cbbad5"
                value={hashLock}
                onChange={(event) => setHashLock(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="hash-receiver">Receiver (Paladin identity)</Label>
              <Input
                id="hash-receiver"
                placeholder="funded_operator@spoke-b-bank-d"
                value={hashReceiver}
                onChange={(event) => setHashReceiver(event.target.value)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="hash-amount">Amount (tCeBM)</Label>
              <Input
                id="hash-amount"
                type="number"
                min="1"
                step="1"
                value={hashAmount}
                onChange={(event) => setHashAmount(event.target.value)}
              />
            </div>

            <Button onClick={onPrepareLockWithHash} disabled={status === "loading"}>
              {status === "loading" ? "Submitting..." : "Review PvP Continuation"}
            </Button>
          </CardContent>
        </Card>
      )}

      {showLockConfirm ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm PvP Transfer</CardTitle>
            <CardDescription>This action starts the PvP settlement process.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm">Agreement ID: {agreementId || "(auto)"}</p>
            <p className="text-sm">Receiver: {lockReceiver}</p>
            <p className="text-sm">Amount: {lockAmount} tCeBM</p>
            <p className="text-sm">Timelock: {timeLock ?? "(backend default)"}</p>
            <div className="flex gap-2">
              <Button onClick={() => void onConfirmLock()} disabled={status === "loading"}>
                {status === "loading" ? "Creating lock..." : "Confirm PvP Transfer"}
              </Button>
              <Button variant="outline" onClick={() => setShowLockConfirm(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {showLockWithHashConfirm ? (
        <Card>
          <CardHeader>
            <CardTitle>Confirm PvP Continuation</CardTitle>
            <CardDescription>This action continues the PvP flow from the receiver side.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm">Settlement Code: {hashLock}</p>
            <p className="text-sm">Receiver: {hashReceiver}</p>
            <p className="text-sm">Amount: {hashAmount} tCeBM</p>
            {hashAgreementId ? <p className="text-sm">Agreement ID: {hashAgreementId}</p> : null}
            <div className="flex gap-2">
              <Button onClick={() => void onConfirmLockWithHash()} disabled={status === "loading"}>
                {status === "loading" ? "Creating lock..." : "Confirm PvP Continuation"}
              </Button>
              <Button variant="outline" onClick={() => setShowLockWithHashConfirm(false)}>
                Cancel
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
