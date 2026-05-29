import type { PoolStatus } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const poolApi = {
  list: (): Promise<PoolStatus[]> => mockDb.getPools(),
};
