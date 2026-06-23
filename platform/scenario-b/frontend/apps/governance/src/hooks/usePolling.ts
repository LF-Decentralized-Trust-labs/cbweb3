// SPDX-License-Identifier: Apache-2.0

import { useEffect, useRef } from "react";

export function usePolling(
  callback: () => void,
  intervalMs: number,
  enabled: boolean,
  maxDurationMs?: number,
): void {
  const callbackRef = useRef(callback);

  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    callbackRef.current();

    const intervalId = window.setInterval(() => {
      callbackRef.current();
    }, intervalMs);

    let timeoutId: number | undefined;
    if (typeof maxDurationMs === "number" && maxDurationMs > 0) {
      timeoutId = window.setTimeout(() => {
        window.clearInterval(intervalId);
      }, maxDurationMs);
    }

    return () => {
      window.clearInterval(intervalId);
      if (timeoutId !== undefined) {
        window.clearTimeout(timeoutId);
      }
    };
  }, [enabled, intervalMs, maxDurationMs]);
}
