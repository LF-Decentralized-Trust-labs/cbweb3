import { useEffect } from "react";

/**
 * Calls `fn` immediately on mount and then every `intervalSeconds` seconds.
 * Cleans up the timer on unmount or when dependencies change.
 */
export function usePolling(fn: () => void, intervalSeconds: number) {
  useEffect(() => {
    fn();
    const id = setInterval(fn, intervalSeconds * 1000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [intervalSeconds]);
}
