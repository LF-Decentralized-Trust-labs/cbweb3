import type { FundingDecisionPayload, FundingRequest } from "../../types";
import { mockDb } from "../mocks/mock-db";

const reviewer = "Treasury Operator";

export const fundingApi = {
  list: (): Promise<FundingRequest[]> => mockDb.getFundingRequests(),
  approve: (payload: FundingDecisionPayload): Promise<FundingRequest> => mockDb.approveFundingRequest(payload, reviewer),
  reject: (payload: FundingDecisionPayload): Promise<FundingRequest> => mockDb.rejectFundingRequest(payload, reviewer),
};
