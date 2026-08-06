// SPDX-License-Identifier: Apache-2.0

import { useCircuitBreakerStore } from "../stores";

// useCircuitBreaker is the single seam through which the chrome reads the network-wide
// breaker condition. AppLayout, Sidebar and DashboardPage all consume it, and all three read
// only `.state` (halted vs operational) plus `fetchState`, so the condition can be re-sourced
// behind this hook without touching them. It now resolves to the derived on-chain condition
// rather than the former mock-capable V1 endpoint — see stores/circuit-breaker.store.ts.
//
// `circuitBreaker` is null when the condition is indeterminate (no pairs, or the pair set is
// only partially readable). Consumers must render that as unknown or operational, never as a
// halt.
export const useCircuitBreaker = () => useCircuitBreakerStore();
