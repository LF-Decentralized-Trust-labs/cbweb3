import type { RelayStatus } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const relayApi = {
  list: (): Promise<RelayStatus[]> => mockDb.getRelays(),
};
