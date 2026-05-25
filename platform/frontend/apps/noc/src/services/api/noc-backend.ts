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
  getAlerts: async (spokeId?: string): Promise<NocAlert[]> => {
    const params = spokeId ? `?spoke_id=${spokeId}` : "";
    const res = await httpClient.get<DataEnvelope<NocAlert[]>>(`/alerts${params}`);
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
