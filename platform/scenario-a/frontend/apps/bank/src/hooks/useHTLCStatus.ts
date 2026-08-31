// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from "react";
import type { HTLCLock } from "../types";
import { htlcApi } from "../services/api";
import { getSecret } from "../services/htlc-secrets";

const normalizeState = (state: string) => state.replace("HTLC_STATE_", "");

export function useHTLCStatus(contractId: string, intervalMs = 5000) {
  const [htlc, setHtlc] = useState<HTLCLock | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!contractId) {
      return;
    }

    let active = true;

    const poll = async () => {
      try {
        const data = await htlcApi.getStatus(contractId);
        if (!active) {
          return;
        }
        const stored = getSecret(contractId);
        setHtlc(stored && !data.secret ? { ...data, secret: stored } : data);
        setLoading(false);

        if (normalizeState(data.state) === "SETTLED" || normalizeState(data.state) === "REFUNDED") {
          return;
        }
      } catch {
        if (!active) {
          return;
        }
      }

      if (active) {
        setTimeout(() => {
          void poll();
        }, intervalMs);
      }
    };

    void poll();

    return () => {
      active = false;
    };
  }, [contractId, intervalMs]);

  return { htlc, loading };
}
