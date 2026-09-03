// SPDX-License-Identifier: Apache-2.0

export type NocEventType = "telemetry.frame" | "telemetry.disconnected" | "pool.threshold.breach";

export type NocEvent = {
  id: string;
  type: NocEventType;
  message: string;
  createdAt: string;
};
