import { mockDb } from "../mocks/mock-db";

export const authApi = {
  login: (username: string, password: string) => mockDb.login(username, password),
  logout: () => mockDb.logout(),
  me: () => mockDb.me(),
};
