// SPDX-License-Identifier: Apache-2.0

import type {
  DisclosureRequest,
  OpenDisclosureRequest,
  SignDisclosureRequest,
} from "../../types/oversight.types";
import { httpClientV2 } from "./http-client";

export const oversightApi = {
  openDisclosure: async (payload: OpenDisclosureRequest): Promise<DisclosureRequest> => {
    const response = await httpClientV2.post<DisclosureRequest>("/oversight/disclosure-request", payload);
    return response.data;
  },
  signDisclosure: async (payload: SignDisclosureRequest): Promise<{ status: string }> => {
    const response = await httpClientV2.post<{ status: string }>("/oversight/disclosure-sign", payload);
    return response.data;
  },
  getDisclosureStatus: async (requestId: string): Promise<DisclosureRequest> => {
    const response = await httpClientV2.get<DisclosureRequest>(`/oversight/disclosure-status/${requestId}`);
    return response.data;
  },
};
