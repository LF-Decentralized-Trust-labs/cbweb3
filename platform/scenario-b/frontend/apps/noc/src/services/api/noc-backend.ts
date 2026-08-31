// SPDX-License-Identifier: Apache-2.0

import type { NocAlert, NocAlertDetail, NocComponent, NocContainerLog, NocHealthEvent, NocSpoke } from "../../types";
import { httpClient } from "./http-client";

type DataEnvelope<T> = { data: T };

export const nocBackendApi = {
  // ── Spokes ──────────────────────────────────────────────────────────────
  getSpokes: async (): Promise<NocSpoke[]> => {
    const res = await httpClient.get<DataEnvelope<NocSpoke[]>>("/spokes");
    return res.data.data;
  },

  // ── Components ──────────────────────────────────────────────────────────
  getComponents: async (spokeId: string): Promise<NocComponent[]> => {
    const res = await httpClient.get<DataEnvelope<NocComponent[]>>(`/spokes/${spokeId}/components`);
    return res.data.data;
  },

  getComponentHealth: async (componentId: string, limit = 100): Promise<NocHealthEvent[]> => {
    const res = await httpClient.get<DataEnvelope<NocHealthEvent[]>>(
      `/components/${componentId}/health?limit=${limit}`,
    );
    return res.data.data;
  },

  // ── Logs ────────────────────────────────────────────────────────────────
  getComponentLogs: async (componentId: string, tail = 200): Promise<NocContainerLog[]> => {
    const res = await httpClient.get<DataEnvelope<NocContainerLog[]>>(
      `/components/${componentId}/logs?tail=${tail}`,
    );
    return res.data.data;
  },

  // ── Alerts ──────────────────────────────────────────────────────────────
  getAlerts: async (spokeId?: string, state?: string): Promise<NocAlert[]> => {
    const params = new URLSearchParams();
    if (spokeId) params.set("spoke_id", spokeId);
    if (state) params.set("state", state);
    const query = params.toString() ? `?${params.toString()}` : "";
    const res = await httpClient.get<DataEnvelope<NocAlert[]>>(`/alerts${query}`);
    return res.data.data;
  },

  getAlert: async (id: string): Promise<NocAlertDetail> => {
    const res = await httpClient.get<DataEnvelope<NocAlertDetail>>(`/alerts/${id}`);
    return res.data.data;
  },

  acknowledgeAlert: async (id: string): Promise<void> => {
    await httpClient.post(`/alerts/${id}/acknowledge`);
  },

  dismissAlert: async (id: string): Promise<void> => {
    await httpClient.post(`/alerts/${id}/dismiss`);
  },
};
