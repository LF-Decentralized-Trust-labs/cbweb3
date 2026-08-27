// SPDX-License-Identifier: Apache-2.0

export type Role = "SYS_ADMIN";

export type SysAdminUser = {
  id: string;
  name: string;
  /**
   * The realm roles the token carries. `role` below is a fixed label, not a claim:
   * every session mapped here became "SYS_ADMIN" whatever its token said.
   */
  roles: string[];
  role: Role;
  institutionId: string;
};

export type LoginRequest = {
  username: string;
  password: string;
};

export type LoginResponse = {
  user: SysAdminUser;
};
