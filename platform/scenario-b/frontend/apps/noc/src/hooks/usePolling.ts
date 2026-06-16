import { useEffect, useRef } from "react";

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
