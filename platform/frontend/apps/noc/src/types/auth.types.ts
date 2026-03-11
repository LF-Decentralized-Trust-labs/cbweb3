export type Role = "SYS_ADMIN";

export type SysAdminUser = {
  id: string;
  name: string;
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
