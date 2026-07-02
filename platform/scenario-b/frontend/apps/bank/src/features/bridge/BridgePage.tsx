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
} from "@cbweb3/ui";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";
import { usePolling } from "../../hooks/usePolling";
import { BRIDGE_STATE } from "../../types/bridge.types";
import { useBridgeStore } from "./bridge.store";

type BadgeVariant = "default" | "secondary" | "outline" | "destructive" | "success" | "warning";

function bridgeBadgeVariant(state: string): BadgeVariant {
  if (state === BRIDGE_STATE.ACTIVE) return "success";
  if (state === BRIDGE_STATE.BURNING) return "warning";
  if (state === BRIDGE_STATE.BURNED) return "secondary";
  if (state === BRIDGE_STATE.RELEASED) return "outline";
  if (state === BRIDGE_STATE.RECONCILIATION_REQUIRED) return "destructive";
  return "default";
}

export function BridgePage() {
  const positions = useBridgeStore((state) => state.positions);
  const status = useBridgeStore((state) => state.status);
  const error = useBridgeStore((state) => state.error);
  const loadPositions = useBridgeStore((state) => state.loadPositions);
  const submitLockMint = useBridgeStore((state) => state.submitLockMint);
  const submitBurnUnlock = useBridgeStore((state) => state.submitBurnUnlock);

  const [amount, setAmount] = useState("");

  const [positionId, setPositionId] = useState("");
  const [stateFilter, setStateFilter] = useState("ALL");
  const [nowMs, setNowMs] = useState(() => Date.now());

  const hasNonTerminal = useMemo(
    () =>
      positions.some(
        (position) =>
          position.bridge_state === BRIDGE_STATE.LOCKING ||
          position.bridge_state === BRIDGE_STATE.BURNING,
      ),
    [positions],
  );

  useEffect(() => {
    void loadPositions();
  }, [loadPositions]);

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);
    return () => {
      window.clearInterval(intervalId);
    };
  }, []);

  const pollingTimedOut = useMemo(() => {
    if (!hasNonTerminal) {
      return false;
    }
    return positions.some((position) => {
      const isTimeoutCandidate =
        position.bridge_state === BRIDGE_STATE.LOCKING ||
        position.bridge_state === BRIDGE_STATE.BURNING;
      if (!isTimeoutCandidate) {
        return false;
      }
      const updatedAtMs = Date.parse(position.updated_at);
      if (Number.isNaN(updatedAtMs)) {
        return false;
      }
      return nowMs - updatedAtMs > 120_000;
    });
  }, [hasNonTerminal, nowMs, positions]);

  usePolling(
    () => {
      void loadPositions();
    },
    5000,
    hasNonTerminal && !pollingTimedOut,
    120_000,
  );

  const filteredPositions = useMemo(() => {
    if (stateFilter === "ALL") {
      return positions;
    }
    return positions.filter((position) => position.bridge_state === stateFilter);
  }, [positions, stateFilter]);

  const selectedPosition = useMemo(
    () => positions.find((position) => position.position_id === positionId),
    [positionId, positions],
  );
  const canBurnUnlock = selectedPosition?.bridge_state === BRIDGE_STATE.ACTIVE;

  const handleLockMint = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!/^\d+$/.test(amount) || Number(amount) <= 0) {
      return;
    }
    await submitLockMint({
      amount,
    });
    setAmount("");
    void loadPositions();
  };

  const handleBurnUnlock = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submitBurnUnlock({ position_id: positionId });
    setPositionId("");
    void loadPositions();
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Lock & Mint</CardTitle>
            <CardDescription>
              Bridge amount into mirrored hub asset. Backend derives bank and asset context from authenticated claims + server config.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleLockMint}>
              <div className="space-y-1">
                <Label htmlFor="amount">Amount</Label>
                <Input
                  id="amount"
                  value={amount}
                  onChange={(event) => setAmount(event.target.value)}
                  inputMode="numeric"
                  pattern="[0-9]+"
                  required
                />
              </div>
              <Button type="submit" disabled={status === "loading"}>
                {status === "loading" ? "Submitting..." : "Submit Lock & Mint"}
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Burn & Unlock</CardTitle>
            <CardDescription>Burn mirrored position and unlock native tCeBM in spoke.</CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={handleBurnUnlock}>
              <div className="space-y-1">
                <Label htmlFor="position_id">Position ID</Label>
                <Input id="position_id" value={positionId} onChange={(event) => setPositionId(event.target.value)} required />
              </div>
              {positionId && !selectedPosition ? (
                <p className="text-xs text-muted-foreground">Enter a valid position ID from the table below to continue.</p>
              ) : null}
              {selectedPosition && !canBurnUnlock ? (
                <p className="text-xs text-muted-foreground">
                  Burn & Unlock is available only when position state is ACTIVE. Current state: {selectedPosition.bridge_state}
                </p>
              ) : null}
              <Button type="submit" variant="outline" disabled={status === "loading" || !canBurnUnlock}>
                {status === "loading" ? "Submitting..." : "Submit Burn & Unlock"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <CardTitle>Bridge Positions</CardTitle>
            <CardDescription>Track bridge lifecycle and relayer progress.</CardDescription>
          </div>
          <div className="w-full md:w-64">
            <Select value={stateFilter} onValueChange={setStateFilter}>
              <SelectTrigger>
                <SelectValue placeholder="Filter by state" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ALL">All States</SelectItem>
                {Object.values(BRIDGE_STATE).map((state) => (
                  <SelectItem key={state} value={state}>
                    {state}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Position ID</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Spoke</TableHead>
                <TableHead>Native Asset</TableHead>
                <TableHead>State</TableHead>
                <TableHead>Mirrored Amount</TableHead>
                <TableHead>Relayer Retries</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredPositions.map((position) => {
                const isTimeoutCandidate =
                  position.bridge_state === BRIDGE_STATE.LOCKING ||
                  position.bridge_state === BRIDGE_STATE.BURNING;

                return (
                  <TableRow key={position.position_id}>
                    <TableCell className="font-mono text-xs">{position.position_id}</TableCell>
                    <TableCell>{position.owner_bank_id}</TableCell>
                    <TableCell>{position.spoke_network}</TableCell>
                    <TableCell>{position.native_asset}</TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1">
                        <Badge variant={bridgeBadgeVariant(position.bridge_state)}>{position.bridge_state}</Badge>
                        {pollingTimedOut && isTimeoutCandidate ? (
                          <span className="text-xs text-muted-foreground">Polling timeout reached. Refresh to continue.</span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>{position.mirrored_amount}</TableCell>
                    <TableCell>{position.relayer_retries}</TableCell>
                  </TableRow>
                );
              })}
              {filteredPositions.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-center text-muted-foreground">
                    No bridge positions yet.
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
          <div className="mt-3 flex items-center gap-2">
            <Button type="button" variant="outline" size="sm" onClick={() => void loadPositions()}>
              Manual Refresh
            </Button>
              <span className="text-xs text-muted-foreground">Use this when a position is still waiting for ACTIVE/BURNED/RELEASED updates.</span>
          </div>
          {error ? <p className="mt-3 text-sm text-destructive">{error}</p> : null}
        </CardContent>
      </Card>
    </div>
  );
}
