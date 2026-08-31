// SPDX-License-Identifier: Apache-2.0

import type { SupervisorEvent } from "../../types";

type EventSubscriber = (event: SupervisorEvent) => void;

class MockSSEService {
  private timer: ReturnType<typeof setInterval> | null = null;

  private subscribers = new Set<EventSubscriber>();

  connect() {
    if (this.timer) {
      return;
    }

    this.timer = setInterval(() => {
      const event: SupervisorEvent = {
        id: `evt_${Math.random().toString(36).slice(2, 10)}`,
        type: Math.random() > 0.66 ? "HTLC_TIMEOUT" : Math.random() > 0.5 ? "AUDIT_ACTIVITY" : "REGISTRY_STATUS",
        severity: Math.random() > 0.7 ? "CRITICAL" : "HIGH",
        message:
          Math.random() > 0.66
            ? "HTLC approaching timeout — lock expiry within threshold window"
            : Math.random() > 0.5
              ? "New compliance audit activity recorded"
              : "Compliance registry status update received",
        createdAt: new Date().toISOString(),
      };
      this.subscribers.forEach((subscriber) => subscriber(event));
    }, 7000);
  }

  disconnect() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  subscribe(subscriber: EventSubscriber) {
    this.subscribers.add(subscriber);
    return () => {
      this.subscribers.delete(subscriber);
    };
  }
}

export const sseService = new MockSSEService();
