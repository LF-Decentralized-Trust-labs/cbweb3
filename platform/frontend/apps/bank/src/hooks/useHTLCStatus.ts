import { useEffect, useState } from "react";
import type { HTLCLock } from "../types";
import { htlcApi } from "../services/api";

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
        setHtlc(data);
        setLoading(false);

        if (data.state === "HTLC_STATE_SETTLED" || data.state === "HTLC_STATE_REFUNDED") {
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
