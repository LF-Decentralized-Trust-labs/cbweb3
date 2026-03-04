import type { UserRole } from "./common.types";

export interface User {
  id: string;
  name: string;
  institutionId: string;
  role: UserRole;
  walletAddress?: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  user: User;
}

export interface WalletBindRequest {
  walletAddress: string;
}
