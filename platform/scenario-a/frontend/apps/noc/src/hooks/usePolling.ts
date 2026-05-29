import { useEffect, useRef } from "react";

/**
 * Calls `fn` immediately on mount and then every `intervalSeconds` seconds.
 * Always uses the latest version of `fn` (via ref) to avoid stale closures.
 * Cleans up the timer on unmount or when `intervalSeconds` changes.
 */
export function usePolling(fn: () => void, intervalSeconds: number) {
  const fnRef = useRef(fn);
  useEffect(() => {
    fnRef.current = fn;
  });

  useEffect(() => {
    const tick = () => fnRef.current();
    tick();
    const id = setInterval(tick, intervalSeconds * 1000);
    return () => clearInterval(id);
  }, [intervalSeconds]);
}
