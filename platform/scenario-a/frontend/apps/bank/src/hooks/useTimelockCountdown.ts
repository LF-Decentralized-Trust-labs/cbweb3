// SPDX-License-Identifier: Apache-2.0

import { useEffect, useMemo, useState } from "react";

export function useTimelockCountdown(timeLock: number) {
  const [secondsLeft, setSecondsLeft] = useState(() => timeLock - Math.floor(Date.now() / 1000));

  useEffect(() => {
    const id = setInterval(() => {
      setSecondsLeft(timeLock - Math.floor(Date.now() / 1000));
    }, 1000);

    return () => {
      clearInterval(id);
    };
  }, [timeLock]);

  const isExpired = secondsLeft <= 0;
  const display = useMemo(() => {
    const safeSeconds = Math.max(0, secondsLeft);
    const hours = Math.floor(safeSeconds / 3600)
      .toString()
      .padStart(2, "0");
    const minutes = Math.floor((safeSeconds % 3600) / 60)
      .toString()
      .padStart(2, "0");
    const seconds = Math.floor(safeSeconds % 60)
      .toString()
      .padStart(2, "0");
    return `${hours}:${minutes}:${seconds}`;
  }, [secondsLeft]);

  return { secondsLeft, isExpired, display };
}
