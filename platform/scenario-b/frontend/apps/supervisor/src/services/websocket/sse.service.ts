import type { SupervisorEvent } from "../../types";

type EventSubscriber = (event: SupervisorEvent) => void;

class NoopEventService {
  connect() {}
  disconnect() {}
  subscribe(_: EventSubscriber) {
    return () => {};
  }
}

export const sseService = new NoopEventService();
