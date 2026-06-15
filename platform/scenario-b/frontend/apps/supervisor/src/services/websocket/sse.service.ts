type EventSubscriber = () => void;

class NoopEventService {
  connect() {}
  disconnect() {}
  subscribe(_: EventSubscriber) {
    return () => {};
  }
}

export const sseService = new NoopEventService();
