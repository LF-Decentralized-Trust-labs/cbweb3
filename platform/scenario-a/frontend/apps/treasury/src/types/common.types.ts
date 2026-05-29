export type AsyncStatus = "idle" | "loading" | "error";

export type ApiResponse<T> = {
  data: T;
};

export type ErrorEnvelope = {
  message: string;
  code?: string;
};
