import { mockDb } from "../mocks/mock-db";

export const authApi = {
  login: (username: string, password: string) => mockDb.login(username, password),
  bindWallet: (walletAddress: string) => mockDb.bindWallet(walletAddress),
  logout: () => mockDb.logout(),
  me: async () => ({ user: mockDb.currentUser }),
};
