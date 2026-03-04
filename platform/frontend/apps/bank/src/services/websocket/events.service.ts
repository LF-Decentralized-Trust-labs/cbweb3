import type { BankEvent, EventType } from "../../types";

type Subscriber = (event: BankEvent) => void;

const randomEvent = (): BankEvent => {
  const types: EventType[] = ["htlc.locked", "htlc.settled", "token.minted", "token.transferred", "amm.pool.updated"];
  return {
    id: `evt_${Math.random().toString(36).slice(2, 10)}`,
    type: types[Math.floor(Math.random() * types.length)],
    occurredAt: new Date().toISOString(),
    payload: {},
  };
};

class MockEventService {
  private subscribers = new Set<Subscriber>();
  private timer?: ReturnType<typeof setInterval>;

  connect() {
    if (this.timer) return;
    this.timer = setInterval(() => {
      const event = randomEvent();
      this.subscribers.forEach((subscriber) => subscriber(event));
    }, 8000);
  }

  disconnect() {
    if (!this.timer) return;
    clearInterval(this.timer);
    this.timer = undefined;
  }

  subscribe(subscriber: Subscriber) {
    this.subscribers.add(subscriber);
    return () => {
      this.subscribers.delete(subscriber);
    };
  }
}

export const eventsService = new MockEventService();
