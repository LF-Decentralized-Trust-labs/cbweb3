import type { LoginResponse } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const authApi = {
  login: (username: string, password: string): Promise<LoginResponse> => mockDb.login(username, password),
  me: (): Promise<LoginResponse> => mockDb.me(),
  logout: () => mockDb.logout(),
};
