// SPDX-License-Identifier: Apache-2.0

const KEY = "htlc_secrets";

function load(): Record<string, string> {
  try {
    return JSON.parse(sessionStorage.getItem(KEY) ?? "{}") as Record<string, string>;
  } catch {
    return {};
  }
}

export function saveSecret(contractId: string, secret: string): void {
  const map = load();
  map[contractId] = secret;
  sessionStorage.setItem(KEY, JSON.stringify(map));
}

export function getSecret(contractId: string): string | undefined {
  return load()[contractId];
}
