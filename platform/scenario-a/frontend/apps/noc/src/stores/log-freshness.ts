// SPDX-License-Identifier: Apache-2.0

// Age of the container-log snapshots shown by the Log Viewer. Logs are collected by the
// spoke's NOC agent and pushed to the backend, so when a container stops (or its agent
// does) the viewer keeps serving the last snapshot with nothing else changing on screen.

/** Log snapshots older than this are flagged; the viewer itself reloads every 15s. */
export const LOG_STALE_SECONDS = 60;

/** Age in seconds of the newest log line, or null when there is nothing to date. */
export function snapshotAgeSeconds(newestOccurredAt: string | undefined, nowMs: number): number | null {
  if (!newestOccurredAt) return null;
  const collectedAt = Date.parse(newestOccurredAt);
  if (Number.isNaN(collectedAt)) return null;
  return Math.max(0, Math.round((nowMs - collectedAt) / 1000));
}
