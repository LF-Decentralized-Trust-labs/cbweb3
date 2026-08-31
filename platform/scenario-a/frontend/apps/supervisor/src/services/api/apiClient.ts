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
  return requestWithAuthRetry<T>(path, init, false);
}

// requestWithAuthRetry issues the request and, on a 401, attempts ONE silent token
// refresh (POST /auth/refresh with the HttpOnly refresh_token cookie) before
// retrying the original request. If the refresh fails the session has truly ended,
// so the auth store is reset. /auth/refresh and /auth/login are never themselves
// retried — that would loop or mask bad credentials.
async function requestWithAuthRetry<T>(
  path: string,
  init: RequestInit | undefined,
  retried: boolean,
): Promise<T> {
  const resp = await fetch(`${BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-Correlation-Id": newCorrelationId(),
      ...(init?.headers ?? {}),
    },
  });

  if (
    resp.status === 401 &&
    !retried &&
    !path.includes("/auth/refresh") &&
    !path.includes("/auth/login")
  ) {
    if (await tryRefreshToken()) {
      return requestWithAuthRetry<T>(path, init, true);
    }
    const { useAuthStore } = await import("../../stores");
    useAuthStore.getState().forceLogout();
  }

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

// tryRefreshToken performs a single silent refresh; true on success.
async function tryRefreshToken(): Promise<boolean> {
  try {
    const resp = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        "X-Correlation-Id": newCorrelationId(),
      },
      body: "{}",
    });
    return resp.ok;
  } catch {
    return false;
  }
}
