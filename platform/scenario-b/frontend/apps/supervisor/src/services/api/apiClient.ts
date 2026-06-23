// SPDX-License-Identifier: Apache-2.0

const BASE_URL = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "";

function newCorrelationId(): string {
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export class ApiError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(`${BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-Correlation-Id": newCorrelationId(),
      ...(init?.headers ?? {}),
    },
  });

  if (!resp.ok) {
    let message = resp.statusText;
    try {
      const body = (await resp.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // ignore parse failure
    }
    throw new ApiError(resp.status, message);
  }

  return resp.json() as Promise<T>;
}
