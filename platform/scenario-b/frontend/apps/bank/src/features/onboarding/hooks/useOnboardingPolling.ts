// SPDX-License-Identifier: Apache-2.0

import { useCallback, useEffect, useState } from "react";
import type { AsyncStatus } from "../../../types";
import { useOnboardingStore } from "../store/useOnboardingStore";

type UseOnboardingPollingResult = {
  pollingStatus: AsyncStatus;
  elapsedSeconds: number;
  refresh: () => Promise<void>;
  stopPolling: () => void;
};

export function useOnboardingPolling(enabled: boolean): UseOnboardingPollingResult {
  const fetchStatus = useOnboardingStore((state) => state.fetchStatus);
  const requestStatus = useOnboardingStore((state) => state.requestStatus);
  const startedAt = useOnboardingStore((state) => state.startedAt);

  const [pollingStatus, setPollingStatus] = useState<AsyncStatus>("idle");
  const [now, setNow] = useState(() => Date.now());
  const [active, setActive] = useState(true);

  const canPoll = enabled && active && (requestStatus === "PENDING" || requestStatus === "CREDENTIAL_REQUESTED");
  const elapsedSeconds = canPoll && startedAt ? Math.floor((now - startedAt) / 1000) : 0;

  const refresh = useCallback(async () => {
    if (!canPoll) return;

    setPollingStatus("loading");
    const response = await fetchStatus();
    setPollingStatus(response ? "idle" : "error");
  }, [canPoll, fetchStatus]);

  const stopPolling = useCallback(() => {
    setActive(false);
  }, []);

  useEffect(() => {
    if (!canPoll) return;

    const initialRefresh = window.setTimeout(() => {
      void refresh();
    }, 0);

    const pollingInterval = window.setInterval(() => {
      void refresh();
    }, 5_000);

    return () => {
      window.clearTimeout(initialRefresh);
      window.clearInterval(pollingInterval);
    };
  }, [canPoll, refresh]);

  useEffect(() => {
    if (!canPoll || !startedAt) return;

    const timerInterval = window.setInterval(() => {
      setNow(Date.now());
    }, 1_000);

    return () => window.clearInterval(timerInterval);
  }, [canPoll, startedAt]);

  return {
    pollingStatus,
    elapsedSeconds,
    refresh,
    stopPolling,
  };
}