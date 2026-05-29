export type AsyncStatus = "idle" | "loading" | "error";

export type ApiResponse<T> = {
  data: T;
};

export type ErrorEnvelope = {
  message: string;
  code?: string;
};

export type PaginationMeta = {
  page: number;
  pageSize: number;
  total: number;
};
